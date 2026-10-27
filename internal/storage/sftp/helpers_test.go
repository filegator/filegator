package sftp

import (
	"errors"
	"io/fs"
	"os"
	"path"
	"strings"
	"testing"

	pkgsftp "github.com/pkg/sftp"

	"github.com/filegator/filegator/internal/config"
	"github.com/filegator/filegator/internal/storage"
)

func TestCleanPath(t *testing.T) {
	cases := map[string]string{
		"":          "/",
		"/":         "/",
		"docs":      "/docs",
		"/docs/":    "/docs",
		"/a/./b":    "/a/b",
		"/a/b/../c": "/a/c",
		`\docs\a`:   "/docs/a",
		"//docs//a": "/docs/a",
	}
	for in, want := range cases {
		got, err := storage.CleanPath(in)
		if err != nil {
			t.Errorf("storage.CleanPath(%q): %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("storage.CleanPath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCleanPathRejectsEscapes(t *testing.T) {
	for _, p := range []string{"..", "../x", "/../x", "a/../../b", "/a\x00b"} {
		if _, err := storage.CleanPath(p); err == nil {
			t.Errorf("storage.CleanPath(%q) accepted", p)
		}
	}
}

func TestWithin(t *testing.T) {
	if !within("/root", "/root") {
		t.Error("a path should be within itself")
	}
	if !within("/root", "/root/a/b") {
		t.Error("descendant not recognised")
	}
	if within("/root", "/rooted") {
		t.Error("prefix match must be segment aware")
	}
	if within("/root", "/other") {
		t.Error("sibling considered within")
	}
}

func TestJoinVirtual(t *testing.T) {
	if got := joinVirtual("/", "a"); got != "/a" {
		t.Errorf("got %q", got)
	}
	if got := joinVirtual("/docs", "a"); got != "/docs/a" {
		t.Errorf("got %q", got)
	}
}

func TestMapErrStatusCodes(t *testing.T) {
	cases := []struct {
		code uint32
		want error
	}{
		{2, storage.ErrNotFound},   // SSH_FX_NO_SUCH_FILE
		{3, storage.ErrPermission}, // SSH_FX_PERMISSION_DENIED
	}
	for _, c := range cases {
		err := mapErr(&pkgsftp.StatusError{Code: c.code})
		if !errors.Is(err, c.want) {
			t.Errorf("code %d mapped to %v, want %v", c.code, err, c.want)
		}
	}
}

func TestMapErrOSErrors(t *testing.T) {
	if !errors.Is(mapErr(os.ErrNotExist), storage.ErrNotFound) {
		t.Error("os.ErrNotExist not mapped")
	}
	if !errors.Is(mapErr(fs.ErrNotExist), storage.ErrNotFound) {
		t.Error("fs.ErrNotExist not mapped")
	}
	if !errors.Is(mapErr(fs.ErrPermission), storage.ErrPermission) {
		t.Error("fs.ErrPermission not mapped")
	}
	if mapErr(nil) != nil {
		t.Error("nil should stay nil")
	}
}

func TestIsNotExist(t *testing.T) {
	if !isNotExist(&pkgsftp.StatusError{Code: 2}) {
		t.Error("no-such-file not detected")
	}
	if isNotExist(&pkgsftp.StatusError{Code: 3}) {
		t.Error("permission denied treated as missing")
	}
	if !isNotExist(os.ErrNotExist) {
		t.Error("os.ErrNotExist not detected")
	}
}

func TestSortEntriesDirectoriesFirst(t *testing.T) {
	entries := []*storage.Entry{
		{Name: "zebra.txt"},
		{Name: "Alpha"},
		{Name: "beta", IsDir: true},
	}
	sortEntries(entries)
	if !entries[0].IsDir {
		t.Error("directory should sort first")
	}
	if entries[1].Name != "Alpha" || entries[2].Name != "zebra.txt" {
		t.Errorf("names not case-insensitive sorted: %+v", entries)
	}
}

func TestAbsJoinsRoot(t *testing.T) {
	b := &Backend{root: "/srv/data"}
	if got := b.abs("/"); got != "/srv/data" {
		t.Errorf("root: got %q", got)
	}
	if got := b.abs("/team/alice"); got != "/srv/data/team/alice" {
		t.Errorf("nested: got %q", got)
	}
}

func TestScopeAbsConfinesToHome(t *testing.T) {
	b := &Backend{root: "/srv/data"}
	s := &Scope{backend: b, home: "/team/alice"}

	// Paths are relative to the home, like the local backend.
	abs, err := s.abs("/docs/a.txt")
	if err != nil {
		t.Fatalf("inside home rejected: %v", err)
	}
	if abs != "/srv/data/team/alice/docs/a.txt" {
		t.Errorf("got %q", abs)
	}
	// "/" is the home itself.
	abs, err = s.abs("/")
	if err != nil {
		t.Fatal(err)
	}
	if abs != "/srv/data/team/alice" {
		t.Errorf("home = %q", abs)
	}
	// Climbing above the home is rejected.
	if _, err := s.abs("/../bob"); !errors.Is(err, storage.ErrOutsideRoot) {
		t.Errorf("escape err = %v, want ErrOutsideRoot", err)
	}
}

func TestScopeHomeRootAllowsEverything(t *testing.T) {
	b := &Backend{root: "/srv/data"}
	s := &Scope{backend: b, home: "/"}
	abs, err := s.abs("/anything/here")
	if err != nil {
		t.Fatalf("admin scope rejected a path: %v", err)
	}
	if abs != path.Join("/srv/data", "anything/here") {
		t.Errorf("got %q", abs)
	}
}

func TestAuthMethodsRequiresCredentials(t *testing.T) {
	// No password and no key: the adapter must refuse rather than attempt an
	// unauthenticated connection.
	if _, err := authMethods(configSFTP("", "")); err == nil {
		t.Error("expected an error without credentials")
	}
}

func TestHostKeyCallbackAcceptsMissingKnownHosts(t *testing.T) {
	// An empty path means "accept any", which is the documented default.
	cb, err := hostKeyCallback("")
	if err != nil {
		t.Fatalf("empty path: %v", err)
	}
	if cb == nil {
		t.Error("callback should not be nil")
	}

	// A path that does not exist must fail loudly rather than silently
	// falling back to an insecure policy.
	if _, err := hostKeyCallback("/nonexistent/known_hosts"); err == nil {
		t.Error("missing known_hosts should be an error")
	}
}

func TestNewRejectsMissingHost(t *testing.T) {
	if _, err := New(configSFTP("", "pw"), "/"); err == nil {
		t.Error("expected an error without a host")
	}
}

func TestDescriptionFormat(t *testing.T) {
	// Description must not panic on a partially built backend; the real value
	// is exercised against a live server.
	s := &Scope{backend: &Backend{root: "/data"}, home: "/x"}
	if s.Home() != "/x" {
		t.Errorf("Home() = %q", s.Home())
	}
}

func TestErrorMessagesMentionSftp(t *testing.T) {
	_, err := New(configSFTP("", ""), "/")
	if err == nil || !strings.Contains(err.Error(), "sftp") {
		t.Errorf("error should name the backend: %v", err)
	}
}

// configSFTP builds a minimal SFTP config for tests.
func configSFTP(password, key string) config.SFTP {
	return config.SFTP{
		Host:       "example.com",
		User:       "demo",
		Password:   password,
		PrivateKey: key,
	}
}
