package sftp

import (
	"crypto/rand"
	"crypto/rsa"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	pkgsftp "github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"

	"github.com/filegator/filegator/internal/config"
	"github.com/filegator/filegator/internal/storage"
)

// testServer is an in-process SSH + SFTP server backed by a temporary
// directory, so the adapter is exercised over a real protocol connection.
type testServer struct {
	t        *testing.T
	listener net.Listener
	hostKey  ssh.Signer
	root     string
	done     chan struct{}
}

// startServer launches a server and returns it with its port.
func startServer(t *testing.T, root string) *testServer {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate host key: %v", err)
	}
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		t.Fatalf("signer: %v", err)
	}

	srv := &testServer{t: t, hostKey: signer, root: root, done: make(chan struct{})}

	cfg := &ssh.ServerConfig{
		PasswordCallback: func(conn ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			if conn.User() == "demo" && string(pass) == "secret" {
				return nil, nil
			}
			return nil, errAuth
		},
	}
	cfg.AddHostKey(signer)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	srv.listener = listener

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				close(srv.done)
				return
			}
			go srv.serve(conn, cfg)
		}
	}()

	t.Cleanup(func() {
		listener.Close()
		select {
		case <-srv.done:
		case <-time.After(2 * time.Second):
		}
	})
	return srv
}

// mustCreate creates an empty file, failing the test on error.
func mustCreate(t *testing.T, fsys storage.FS, path string) {
	t.Helper()
	w, err := fsys.Create(path)
	if err != nil {
		t.Fatalf("Create %s: %v", path, err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close %s: %v", path, err)
	}
}

// errAuth is returned when the password does not match.
var errAuth = &authError{}

type authError struct{}

func (e *authError) Error() string { return "authentication failed" }

// serve handshakes one connection and serves SFTP on it.
func (s *testServer) serve(conn net.Conn, cfg *ssh.ServerConfig) {
	sshConn, chans, reqs, err := ssh.NewServerConn(conn, cfg)
	if err != nil {
		conn.Close()
		return
	}
	defer sshConn.Close()
	go ssh.DiscardRequests(reqs)

	for newChan := range chans {
		if newChan.ChannelType() != "session" {
			newChan.Reject(ssh.UnknownChannelType, "only sessions")
			continue
		}
		channel, requests, err := newChan.Accept()
		if err != nil {
			continue
		}
		go func() {
			defer channel.Close()
			for req := range requests {
				if req.Type != "subsystem" || len(req.Payload) < 4 || string(req.Payload[4:]) != "sftp" {
					req.Reply(false, nil)
					continue
				}
				req.Reply(true, nil)
				// Serve with the server-side helper, rooted at the temp dir.
				server, err := pkgsftp.NewServer(channel, pkgsftp.WithServerWorkingDirectory(s.root))
				if err != nil {
					return
				}
				server.Serve()
				server.Close()
				return
			}
		}()
	}
}

// port returns the listening port.
func (s *testServer) port() int {
	return s.listener.Addr().(*net.TCPAddr).Port
}

// newTestBackend starts a server and connects the adapter to it.
func newTestBackend(t *testing.T) (*Backend, string) {
	t.Helper()
	root := t.TempDir()
	srv := startServer(t, root)

	// The in-process server serves the real filesystem, so the remote root is
	// the temporary directory itself rather than "/".
	backend, err := New(config.SFTP{
		Host:     "127.0.0.1",
		Port:     srv.port(),
		User:     "demo",
		Password: "secret",
		Timeout:  5,
	}, root)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { backend.Close() })
	return backend, root
}

func TestSFTPReadWrite(t *testing.T) {
	b, _ := newTestBackend(t)
	fsys, err := b.Scope("/")
	if err != nil {
		t.Fatalf("Scope: %v", err)
	}

	if err := fsys.Mkdir("/docs"); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	w, err := fsys.Create("/docs/a.txt")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := w.Write([]byte("hello sftp")); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	r, err := fsys.Open("/docs/a.txt")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer r.Close()
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello sftp" {
		t.Errorf("content = %q", data)
	}
}

func TestSFTPReadDirAndStat(t *testing.T) {
	b, _ := newTestBackend(t)
	fsys, _ := b.Scope("/")
	fsys.Mkdir("/dir")
	mustCreate(t, fsys, "/file.txt")

	entries, err := fsys.ReadDir("/")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("entries = %+v", entries)
	}
	if !entries[0].IsDir || entries[0].Name != "dir" {
		t.Errorf("directories should sort first: %+v", entries)
	}

	entry, err := fsys.Stat("/file.txt")
	if err != nil {
		t.Fatal(err)
	}
	if entry.IsDir || entry.Path != "/file.txt" {
		t.Errorf("entry = %+v", entry)
	}
}

func TestSFTPRenameAndRemove(t *testing.T) {
	b, _ := newTestBackend(t)
	fsys, _ := b.Scope("/")
	mustCreate(t, fsys, "/a.txt")

	if err := fsys.Rename("/a.txt", "/b.txt"); err != nil {
		t.Fatalf("Rename: %v", err)
	}
	if !fsys.Exists("/b.txt") || fsys.Exists("/a.txt") {
		t.Error("rename did not take effect")
	}
	if err := fsys.Remove("/b.txt"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if fsys.Exists("/b.txt") {
		t.Error("file still exists")
	}
}

func TestSFTPRenameRefusesOverwrite(t *testing.T) {
	b, _ := newTestBackend(t)
	fsys, _ := b.Scope("/")
	mustCreate(t, fsys, "/a.txt")
	mustCreate(t, fsys, "/b.txt")

	if err := fsys.Rename("/a.txt", "/b.txt"); err != storage.ErrExists {
		t.Fatalf("got %v, want ErrExists", err)
	}
}

func TestSFTPCopyFile(t *testing.T) {
	b, _ := newTestBackend(t)
	fsys, _ := b.Scope("/")
	w, _ := fsys.Create("/src.txt")
	w.Write([]byte("payload"))
	w.Close()

	if err := fsys.Copy("/src.txt", "/dst.txt"); err != nil {
		t.Fatalf("Copy: %v", err)
	}
	r, err := fsys.Open("/dst.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	data, _ := io.ReadAll(r)
	if string(data) != "payload" {
		t.Errorf("content = %q", data)
	}
}

func TestSFTPCopyDirectory(t *testing.T) {
	b, _ := newTestBackend(t)
	fsys, _ := b.Scope("/")
	fsys.Mkdir("/src")
	w, _ := fsys.Create("/src/a.txt")
	w.Write([]byte("x"))
	w.Close()

	if err := fsys.Copy("/src", "/dst"); err != nil {
		t.Fatalf("Copy dir: %v", err)
	}
	if !fsys.Exists("/dst/a.txt") {
		t.Error("nested file not copied")
	}
}

func TestSFTPScopeConfinement(t *testing.T) {
	b, _ := newTestBackend(t)
	fsys, _ := b.Scope("/")
	fsys.MkdirAll("/team/alice")
	fsys.MkdirAll("/team/bob")
	mustCreate(t, fsys, "/team/bob/secret.txt")

	alice, err := b.Scope("/team/alice")
	if err != nil {
		t.Fatalf("Scope: %v", err)
	}

	if _, err := alice.Open("/../bob/secret.txt"); err == nil {
		t.Error("alice read bob's file")
	}
	if alice.Exists("/../bob/secret.txt") {
		t.Error("alice sees bob's file")
	}
	if _, err := alice.ReadDir("/.."); err == nil {
		t.Error("alice listed outside her home")
	}
	if err := alice.Remove("/../bob/secret.txt"); err == nil {
		t.Error("alice removed bob's file")
	}
}

func TestSFTPMissingRootFailsFast(t *testing.T) {
	root := t.TempDir()
	srv := startServer(t, root)

	if _, err := New(config.SFTP{
		Host:     "127.0.0.1",
		Port:     srv.port(),
		User:     "demo",
		Password: "secret",
		Timeout:  5,
	}, "/does-not-exist"); err == nil {
		t.Fatal("expected an error for a missing remote root")
	}
}

func TestSFTPAuthFailure(t *testing.T) {
	root := t.TempDir()
	srv := startServer(t, root)

	if _, err := New(config.SFTP{
		Host:     "127.0.0.1",
		Port:     srv.port(),
		User:     "demo",
		Password: "wrong",
		Timeout:  5,
	}, "/"); err == nil {
		t.Fatal("expected an authentication error")
	}
}

func TestSFTPEnsureDir(t *testing.T) {
	b, root := newTestBackend(t)

	if err := b.EnsureDir("/new/nested"); err != nil {
		t.Fatalf("EnsureDir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "new", "nested")); err != nil {
		t.Fatalf("directory not created on disk: %v", err)
	}
	if err := b.EnsureDir("/new/nested"); err != storage.ErrExists {
		t.Fatalf("got %v, want ErrExists", err)
	}
}

func TestSFTPCapabilities(t *testing.T) {
	b, _ := newTestBackend(t)
	caps := b.Capabilities()
	if !caps.Chmod {
		t.Error("sftp should support chmod")
	}
	if caps.ServerBrowse {
		t.Error("sftp must not offer the host filesystem picker")
	}
}
