package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// firstStartRequest builds a request for a folder that has never run.
func firstStartRequest(t *testing.T, dir string) InitRequest {
	t.Helper()
	return InitRequest{
		ConfigPath:  filepath.Join(dir, "private", "config.json"),
		UsersFile:   filepath.Join(dir, "private", "users.json"),
		StorageRoot: filepath.Join(dir, "repository"),
	}
}

// yes always answers "y".
func yes(string) (bool, error) { return true, nil }

// no always answers "n".
func no(string) (bool, error) { return false, nil }

func TestNeedsInitOnlyWhenSomethingIsMissing(t *testing.T) {
	dir := t.TempDir()
	req := firstStartRequest(t, dir)
	if !req.NeedsInit() {
		t.Fatal("an empty folder is a first start")
	}

	for _, path := range []string{req.ConfigPath, req.StorageRoot} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if req.NeedsInit() {
		t.Error("a folder with its config and storage root is not a first start")
	}
	// A remote backend has no local root to create.
	remote := req
	remote.StorageRoot = ""
	if remote.NeedsInit() {
		t.Error("a remote backend should not count as a first start")
	}
}

// TestDescribeListsEveryPath is what the operator reads before saying yes: it
// must name the paths, not talk about "some files".
func TestDescribeListsEveryPath(t *testing.T) {
	dir := t.TempDir()
	req := firstStartRequest(t, dir)
	if err := os.MkdirAll(filepath.Dir(req.UsersFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(req.UsersFile+".sample", []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}

	lines := strings.Join(req.Describe(), "\n")
	for _, want := range []string{req.ConfigPath, req.UsersFile, req.StorageRoot} {
		if !strings.Contains(lines, want) {
			t.Errorf("describe does not name %s:\n%s", want, lines)
		}
	}
	// The users file is described as a copy of its template, so nobody is
	// surprised that admin/guest appear.
	if !strings.Contains(lines, "copied from") {
		t.Errorf("describe should say the users file comes from the sample:\n%s", lines)
	}
}

func TestResolveInitAsksOnATerminal(t *testing.T) {
	req := firstStartRequest(t, t.TempDir())

	var out bytes.Buffer
	create, reason := ResolveInit(InitAsk, req, &out, true, yes)
	if !create {
		t.Fatalf("a yes should create; reason %q", reason)
	}
	prompt := out.String()
	if !strings.Contains(prompt, "Create them now? [y/N]") {
		t.Errorf("the prompt should say what the default is:\n%s", prompt)
	}
	if !strings.Contains(prompt, req.ConfigPath) {
		t.Errorf("the prompt should name the paths, so consent is informed:\n%s", prompt)
	}

	out.Reset()
	create, reason = ResolveInit(InitAsk, req, &out, true, no)
	if create {
		t.Error("the default answer must be no")
	}
	if !strings.Contains(reason, "declined") {
		t.Errorf("reason = %q, want it to say the first start was declined", reason)
	}
}

// TestResolveInitRefusesWithoutATerminal matters for service units: a question
// nobody can see would block the start forever.
func TestResolveInitRefusesWithoutATerminal(t *testing.T) {
	req := firstStartRequest(t, t.TempDir())

	var out bytes.Buffer
	create, reason := ResolveInit(InitAsk, req, &out, false, yes)
	if create {
		t.Fatal("without a terminal nothing may be created")
	}
	if out.Len() != 0 {
		t.Errorf("nothing should be printed without a terminal: %q", out.String())
	}
	if !strings.Contains(reason, "-init") {
		t.Errorf("the reason should point at -init: %q", reason)
	}
}

// TestResolveInitInitFlag covers -init, the only flag there is.
func TestResolveInitInitFlag(t *testing.T) {
	req := firstStartRequest(t, t.TempDir())

	create, reason := ResolveInit(InitYes, req, nil, false, no)
	if !create || reason != "" {
		t.Errorf("-init: create=%v reason=%q", create, reason)
	}

	// Declining on the terminal is the only way to say no; there is no flag
	// for it, so an answer of "no" is the whole test.
	var out bytes.Buffer
	create, reason = ResolveInit(InitAsk, req, &out, true, no)
	if create || !strings.Contains(reason, "declined") {
		t.Errorf("a no must create nothing: create=%v reason=%q", create, reason)
	}
}

// TestResolveInitSkipsAWarmStart: an installation with everything in place is
// never asked, and no flag is needed.
func TestResolveInitSkipsAWarmStart(t *testing.T) {
	dir := t.TempDir()
	req := firstStartRequest(t, dir)
	if err := os.MkdirAll(filepath.Dir(req.ConfigPath), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{req.ConfigPath, req.StorageRoot} {
		if err := os.WriteFile(path, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	create, reason := ResolveInit(InitAsk, req, nil, false, no)
	if !create || reason != "" {
		t.Errorf("a warm start should need no decision: create=%v reason=%q", create, reason)
	}
}
