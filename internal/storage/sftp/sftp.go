// Package sftp implements the storage.Storage interface over SFTP.
//
// It mirrors the local backend's semantics: paths are virtual and always start
// with "/", a Scope confines a user to a home folder, and errors map onto the
// shared storage sentinel errors so handlers behave identically.
//
// SFTP has no server-side copy, so Copy downloads to memory and writes back.
// That is correct but slow for large trees; it is the same limitation v7's
// Flysystem adapter had.
package sftp

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	pkgsftp "github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"

	"github.com/filegator/filegator/internal/config"
	"github.com/filegator/filegator/internal/storage"
)

// copyBufferSize bounds how much of a file is held in memory during Copy.
const copyBufferSize = 4 << 20

// dialTimeoutDefault is used when the config does not set one.
const dialTimeoutDefault = 15

// Backend talks to a remote server over SFTP.
type Backend struct {
	root      string
	sshClient *ssh.Client
	client    *pkgsftp.Client
	mu        sync.Mutex
	closed    bool
}

// New connects to the configured server and returns a Backend rooted at
// cfg.Root on the remote machine.
func New(cfg config.SFTP, root string) (*Backend, error) {
	if cfg.Host == "" || cfg.User == "" {
		return nil, errors.New("sftp: host and user are required")
	}
	port := cfg.Port
	if port == 0 {
		port = 22
	}
	timeout := time.Duration(cfg.Timeout) * time.Second
	if cfg.Timeout == 0 {
		timeout = dialTimeoutDefault * time.Second
	}

	auth, err := authMethods(cfg)
	if err != nil {
		return nil, err
	}
	hostKey, err := hostKeyCallback(cfg.KnownHosts)
	if err != nil {
		return nil, err
	}

	sshCfg := &ssh.ClientConfig{
		User:            cfg.User,
		Auth:            auth,
		HostKeyCallback: hostKey,
		Timeout:         timeout,
	}
	addr := fmt.Sprintf("%s:%d", cfg.Host, port)
	sshClient, err := ssh.Dial("tcp", addr, sshCfg)
	if err != nil {
		return nil, fmt.Errorf("sftp: connect to %s: %w", addr, err)
	}

	client, err := pkgsftp.NewClient(sshClient)
	if err != nil {
		sshClient.Close()
		return nil, fmt.Errorf("sftp: start session: %w", err)
	}

	cleanRoot := path.Clean("/" + strings.Trim(root, "/"))
	b := &Backend{root: cleanRoot, sshClient: sshClient, client: client}

	// Fail fast when the root is missing or unreadable, so startup reports a
	// clear error instead of every request failing later.
	if _, err := client.Stat(cleanRoot); err != nil {
		b.Close()
		return nil, fmt.Errorf("sftp: cannot read %s: %w", cleanRoot, err)
	}
	return b, nil
}

// authMethods builds the SSH authentication methods from the configuration.
func authMethods(cfg config.SFTP) ([]ssh.AuthMethod, error) {
	var methods []ssh.AuthMethod
	if cfg.Password != "" {
		methods = append(methods, ssh.Password(cfg.Password))
	}
	if cfg.PrivateKey != "" {
		data, err := os.ReadFile(cfg.PrivateKey)
		if err != nil {
			return nil, fmt.Errorf("sftp: read private key: %w", err)
		}
		signer, err := ssh.ParsePrivateKey(data)
		if err != nil {
			return nil, fmt.Errorf("sftp: parse private key: %w", err)
		}
		methods = append(methods, ssh.PublicKeys(signer))
	}
	if len(methods) == 0 {
		return nil, errors.New("sftp: a password or private key is required")
	}
	return methods, nil
}

// hostKeyCallback returns a known_hosts check when a file is configured, and
// an accept-any policy otherwise.
func hostKeyCallback(knownHostsPath string) (ssh.HostKeyCallback, error) {
	if knownHostsPath == "" {
		return ssh.InsecureIgnoreHostKey(), nil
	}
	callback, err := knownhosts.New(knownHostsPath)
	if err != nil {
		return nil, fmt.Errorf("sftp: read known_hosts: %w", err)
	}
	return callback, nil
}

// Close ends the connection.
func (b *Backend) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil
	}
	b.closed = true
	if b.client != nil {
		b.client.Close()
	}
	if b.sshClient != nil {
		return b.sshClient.Close()
	}
	return nil
}

// Description identifies the remote store for the admin UI.
func (b *Backend) Description() string {
	return "sftp://" + b.sshClient.RemoteAddr().String() + b.root
}

// Capabilities reports what an SFTP server supports. chmod works on Unix
// servers, and the host filesystem picker is meaningless for a remote store.
func (b *Backend) Capabilities() storage.Caps {
	return storage.Caps{Chmod: true, ServerBrowse: false}
}

// Scope returns a confined view rooted at home.
func (b *Backend) Scope(home string) (storage.FS, error) {
	clean, err := storage.CleanPath(home)
	if err != nil {
		return nil, err
	}
	abs := b.abs(clean)
	info, err := b.client.Stat(abs)
	if err != nil {
		if isNotExist(err) {
			return nil, storage.ErrNotFound
		}
		return nil, err
	}
	if !info.IsDir() {
		return nil, storage.ErrNotDir
	}
	return &Scope{backend: b, home: clean}, nil
}

// EnsureDir creates a directory and its parents on the remote server.
func (b *Backend) EnsureDir(p string) error {
	clean, err := storage.CleanPath(p)
	if err != nil {
		return err
	}
	abs := b.abs(clean)
	if _, err := b.client.Stat(abs); err == nil {
		return storage.ErrExists
	}
	if err := b.client.MkdirAll(abs); err != nil {
		if isNotExist(err) {
			return storage.ErrNotFound
		}
		return err
	}
	return nil
}

// abs joins a cleaned virtual path onto the remote root.
func (b *Backend) abs(clean string) string {
	if clean == "/" {
		return b.root
	}
	return path.Join(b.root, strings.TrimPrefix(clean, "/"))
}

// Scope is a confined view of a remote directory.
type Scope struct {
	backend *Backend
	home    string
}

// Home returns the home folder path relative to the storage root.
func (s *Scope) Home() string { return s.home }

// abs resolves a virtual path within the scope.
//
// Paths are interpreted relative to the scope home, exactly like the local
// backend: a scope at /team/alice sees its own files at "/", and ".." cannot
// climb above it.
func (s *Scope) abs(p string) (string, error) {
	clean, err := storage.CleanPath(p)
	if err != nil {
		return "", err
	}
	// Rebasing keeps the two backends consistent: "/" means the home folder.
	full := clean
	if s.home != "/" {
		full = path.Join(s.home, strings.TrimPrefix(clean, "/"))
		if !within(s.home, full) {
			return "", storage.ErrOutsideScope
		}
	}
	return s.backend.abs(full), nil
}

// Stat describes a path.
func (s *Scope) Stat(p string) (*storage.Entry, error) {
	clean, abs, err := s.resolve(p)
	if err != nil {
		return nil, err
	}
	info, err := s.backend.client.Stat(abs)
	if err != nil {
		return nil, mapErr(err)
	}
	return toEntry(clean, info), nil
}

// ReadDir lists a directory.
func (s *Scope) ReadDir(dir string) ([]*storage.Entry, error) {
	clean, abs, err := s.resolve(dir)
	if err != nil {
		return nil, err
	}
	items, err := s.backend.client.ReadDir(abs)
	if err != nil {
		return nil, mapErr(err)
	}
	entries := make([]*storage.Entry, 0, len(items))
	for _, item := range items {
		entries = append(entries, toEntry(joinVirtual(clean, item.Name()), item))
	}
	sortEntries(entries)
	return entries, nil
}

// Open opens a remote file for reading.
func (s *Scope) Open(p string) (io.ReadCloser, error) {
	_, abs, err := s.resolve(p)
	if err != nil {
		return nil, err
	}
	f, err := s.backend.client.Open(abs)
	if err != nil {
		return nil, mapErr(err)
	}
	return f, nil
}

// Create creates a new remote file, failing when it already exists.
func (s *Scope) Create(p string) (io.WriteCloser, error) {
	_, abs, err := s.resolve(p)
	if err != nil {
		return nil, err
	}
	// sftp has no exclusive-create flag, so check first. A racing writer can
	// still slip in between; the SFTP server would then get two handles on
	// the same file rather than losing data silently to a truncated one.
	if _, err := s.backend.client.Stat(abs); err == nil {
		return nil, storage.ErrExists
	}
	f, err := s.backend.client.Create(abs)
	if err != nil {
		return nil, mapErr(err)
	}
	return f, nil
}

// OpenWrite truncates an existing remote file for rewriting.
func (s *Scope) OpenWrite(p string) (io.WriteCloser, error) {
	_, abs, err := s.resolve(p)
	if err != nil {
		return nil, err
	}
	if _, err := s.backend.client.Stat(abs); err != nil {
		return nil, mapErr(err)
	}
	f, err := s.backend.client.Create(abs)
	if err != nil {
		return nil, mapErr(err)
	}
	return f, nil
}

// Mkdir creates one directory.
func (s *Scope) Mkdir(p string) error {
	_, abs, err := s.resolve(p)
	if err != nil {
		return err
	}
	if _, statErr := s.backend.client.Stat(abs); statErr == nil {
		return storage.ErrExists
	}
	if err := s.backend.client.Mkdir(abs); err != nil {
		return mapErr(err)
	}
	return nil
}

// MkdirAll creates a directory and its parents.
func (s *Scope) MkdirAll(p string) error {
	_, abs, err := s.resolve(p)
	if err != nil {
		return err
	}
	if err := s.backend.client.MkdirAll(abs); err != nil {
		return mapErr(err)
	}
	return nil
}

// Rename moves a file or directory.
func (s *Scope) Rename(src, dst string) error {
	_, absSrc, err := s.resolve(src)
	if err != nil {
		return err
	}
	_, absDst, err := s.resolve(dst)
	if err != nil {
		return err
	}
	if _, statErr := s.backend.client.Stat(absDst); statErr == nil {
		return storage.ErrExists
	}
	if err := s.backend.client.Rename(absSrc, absDst); err != nil {
		return mapErr(err)
	}
	return nil
}

// Copy copies a file or directory. SFTP has no server-side copy, so this
// streams the data through the client.
func (s *Scope) Copy(src, dst string) error {
	_, absSrc, err := s.resolve(src)
	if err != nil {
		return err
	}
	_, absDst, err := s.resolve(dst)
	if err != nil {
		return err
	}
	if _, statErr := s.backend.client.Stat(absDst); statErr == nil {
		return storage.ErrExists
	}
	if within(absSrc, absDst) {
		return storage.ErrInvalidPath
	}
	return s.copyTree(absSrc, absDst)
}

// copyTree copies a file or directory recursively.
func (s *Scope) copyTree(src, dst string) error {
	info, err := s.backend.client.Stat(src)
	if err != nil {
		return mapErr(err)
	}
	if !info.IsDir() {
		return s.copyFile(src, dst)
	}
	if err := s.backend.client.Mkdir(dst); err != nil && !isNotExist(err) {
		return mapErr(err)
	}
	items, err := s.backend.client.ReadDir(src)
	if err != nil {
		return mapErr(err)
	}
	for _, item := range items {
		if err := s.copyTree(path.Join(src, item.Name()), path.Join(dst, item.Name())); err != nil {
			return err
		}
	}
	return nil
}

// copyFile streams one file, holding at most copyBufferSize in memory.
func (s *Scope) copyFile(src, dst string) error {
	in, err := s.backend.client.Open(src)
	if err != nil {
		return mapErr(err)
	}
	defer in.Close()

	out, err := s.backend.client.Create(dst)
	if err != nil {
		return mapErr(err)
	}
	if _, err := io.CopyBuffer(out, in, make([]byte, copyBufferSize)); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// Remove deletes a file or directory tree.
func (s *Scope) Remove(p string) error {
	clean, abs, err := s.resolve(p)
	if err != nil {
		return err
	}
	if clean == "/" {
		return storage.ErrOutsideScope
	}
	if err := s.backend.client.RemoveAll(abs); err != nil {
		return mapErr(err)
	}
	return nil
}

// Chmod sets permission bits. Many SFTP servers ignore or reject this.
func (s *Scope) Chmod(p string, mode fs.FileMode) error {
	clean, abs, err := s.resolve(p)
	if err != nil {
		return err
	}
	if clean == "/" {
		return storage.ErrOutsideScope
	}
	if err := s.backend.client.Chmod(abs, mode.Perm()); err != nil {
		return mapErr(err)
	}
	return nil
}

// Exists reports whether a path exists.
func (s *Scope) Exists(p string) bool {
	_, err := s.Stat(p)
	return err == nil
}

// resolve returns the cleaned virtual path and the absolute remote path.
func (s *Scope) resolve(p string) (string, string, error) {
	clean, err := storage.CleanPath(p)
	if err != nil {
		return "", "", err
	}
	abs, err := s.abs(clean)
	if err != nil {
		return "", "", err
	}
	return clean, abs, nil
}
