package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/filegator/filegator/internal/app"
	"github.com/filegator/filegator/internal/version"
)

func captured(t *testing.T, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	var out, errOut bytes.Buffer
	code = run(args, &out, &errOut)
	return out.String(), errOut.String(), code
}

// TestHelpPrintsTheUsageAndExitsZero covers the way a first-time reader asks
// what the binary does. It used to print the bare flag list to stderr and
// exit 2, which reads as a failure.
func TestHelpPrintsTheUsageAndExitsZero(t *testing.T) {
	for _, arg := range []string{"-h", "-help", "--help"} {
		stdout, stderr, code := captured(t, arg)
		if code != 0 {
			t.Errorf("%s exited %d, want 0", arg, code)
		}
		if stderr != "" {
			t.Errorf("%s wrote to stderr, so `filegator -h | less` would show nothing: %q", arg, stderr)
		}
		for _, want := range []string{"Usage:", "filegator [flags]", "-addr", "-config", "-base-path", "-frontend-dir", "-export-frontend", "-reset-admin-password", "Examples:"} {
			if !strings.Contains(stdout, want) {
				t.Errorf("%s: help does not mention %q:\n%s", arg, want, stdout)
			}
		}
	}
}

// TestHelpSaysRunningWithNoFlagsServes: the default action is the one thing a
// reader needs that a flag list cannot say.
func TestHelpSaysRunningWithNoFlagsServes(t *testing.T) {
	stdout, _, _ := captured(t, "-h")
	if !strings.Contains(stdout, "serve the web interface (the default)") {
		t.Fatalf("help does not state that no flags starts the server:\n%s", stdout)
	}
}

// TestUnknownFlagFails: a typo must still be an error, on stderr, with the
// help screen, so the flag list is a hint rather than a surprise.
func TestUnknownFlagFails(t *testing.T) {
	stdout, stderr, code := captured(t, "-nope")
	if code != 2 {
		t.Errorf("exit %d, want 2", code)
	}
	if stdout != "" {
		t.Errorf("unknown flag wrote to stdout: %q", stdout)
	}
	if !strings.Contains(stderr, "flag provided but not defined: -nope") || !strings.Contains(stderr, "Usage:") {
		t.Errorf("stderr does not explain the mistake:\n%s", stderr)
	}
}

// TestStrayArgumentIsAnError: with no flags the server starts, so a forgotten
// dash (-addr as a bare word) would otherwise be ignored and the server would
// listen on the default address instead of the one asked for.
func TestStrayArgumentIsAnError(t *testing.T) {
	_, stderr, code := captured(t, "addr")
	if code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
	if !strings.Contains(stderr, `unexpected argument "addr"`) {
		t.Fatalf("stderr does not name the stray argument:\n%s", stderr)
	}
}

// TestFlagsParseAsDocumented pins the names the docs and the Makefile use.
func TestFlagsParseAsDocumented(t *testing.T) {
	fs, opts := newFlagSet(&bytes.Buffer{})
	err := fs.Parse([]string{"-addr", ":9000", "-config", "/etc/fg.json", "-base-path", "/files", "-frontend-dir", "/srv/ui"})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if opts.addr != ":9000" || opts.configPath != "/etc/fg.json" || opts.basePath != "/files" || opts.frontendDir != "/srv/ui" {
		t.Fatalf("parsed %+v", opts)
	}
}

// TestListAdminsFlagRunsAndExits checks the flag is wired to the command and
// returns 0, so a script can tell success from failure.
func TestListAdminsFlagRunsAndExits(t *testing.T) {
	dir := t.TempDir()
	sample, err := os.ReadFile("private/users.json.sample")
	if err != nil {
		t.Fatal(err)
	}
	usersPath := filepath.Join(dir, "users.json")
	if err := os.WriteFile(usersPath, sample, 0o600); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(dir, "config.json")
	cfg, err := json.Marshal(map[string]any{"auth": map[string]any{"type": "json", "users_file": usersPath}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, cfg, 0o600); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	if code := run([]string{"-config", configPath, "-list-admins"}, &out, &errOut); code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "admin") {
		t.Fatalf("output does not list the admin:\n%s", out.String())
	}
}

// TestListAdminsFailsOnBadConfig checks the failure code: a script must be able
// to see that the listing did not happen.
func TestListAdminsFailsOnBadConfig(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := run([]string{"-config", "/nonexistent/c.json", "-list-admins"}, &out, &errOut); code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if !strings.Contains(errOut.String(), "does not exist") {
		t.Fatalf("stderr says nothing useful: %q", errOut.String())
	}
}

// TestHelpShowsTheVersion checks the release number is on the help screen: it is
// the first thing asked when reporting a problem or checking that an upgrade
// took, and it used to live only in the admin area and version.json.
func TestHelpShowsTheVersion(t *testing.T) {
	stdout, stderr, code := captured(t, "-h")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(stdout, "FileGator "+version.Version) {
		t.Fatalf("the help screen does not name the version %q:\n%s", version.Version, stdout)
	}
	if stderr != "" {
		t.Errorf("the help screen wrote to stderr: %q", stderr)
	}
}

// TestInitFlagsOutsideATerminal covers the container and service-unit case: no
// terminal means the binary must not wait for an answer, and the two flags are
// how it is told which answer to take.
func TestInitFlagsOutsideATerminal(t *testing.T) {
	cases := []struct {
		args  []string
		check func(t *testing.T, opts *options)
	}{
		{[]string{}, func(t *testing.T, o *options) {
			if o.initMode() != app.InitAsk {
				t.Error("no flag should ask")
			}
		}},
		{[]string{"-init"}, func(t *testing.T, o *options) {
			if o.initMode() != app.InitYes {
				t.Error("-init should create without asking")
			}
		}},
	}
	for _, c := range cases {
		fs, opts := newFlagSet(io.Discard)
		if err := fs.Parse(c.args); err != nil {
			t.Fatalf("%v: %v", c.args, err)
		}
		c.check(t, opts)
	}

	// There is no flag to decline: the answer on the terminal is the only way,
	// so -no-init must be an unknown flag rather than a silent alias.
	_, stderr, code := captured(t, "-no-init")
	if code != 2 || !strings.Contains(stderr, "flag provided but not defined") {
		t.Errorf("-no-init should be rejected: exit %d, stderr %q", code, stderr)
	}
}

// TestAskYesNoDefaultsToNo: pressing Enter must create nothing.
func TestAskYesNoDefaultsToNo(t *testing.T) {
	for _, c := range []struct {
		input string
		want  bool
	}{
		{"\n", false},
		{"n\n", false},
		{"y\n", true},
		{"YES\n", true},
		{" y \n", true},
	} {
		withStdin(t, c.input, func() {
			got, err := askYesNo("")
			if err != nil {
				t.Fatalf("%q: %v", c.input, err)
			}
			if got != c.want {
				t.Errorf("answer to %q = %v, want %v", c.input, got, c.want)
			}
		})
	}
}

// withStdin replaces os.Stdin for the duration of fn.
func withStdin(t *testing.T, input string, fn func()) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.WriteString(input); err != nil {
		t.Fatal(err)
	}
	w.Close()
	old := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = old })
	fn()
}

// TestCanAnswer covers the check that decides whether the first-start question
// is asked at all. It replaced a terminal ioctl, which made the binary build
// only on Unix; a file-mode check works everywhere.
func TestCanAnswer(t *testing.T) {
	// /dev/null is what a service unit and a container give: nobody to ask.
	devNull, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer devNull.Close()
	if canAnswer(devNull) {
		t.Error("/dev/null must not be treated as an answerable terminal")
	}

	// A regular file redirect is not a conversation either.
	file, err := os.CreateTemp(t.TempDir(), "redirect")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if canAnswer(file) {
		t.Error("a file redirect must not be treated as a terminal")
	}

	// A pipe is where a script writes "y", so it counts.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()
	if !canAnswer(r) {
		t.Error("a pipe should count as answerable")
	}

	// And a closed stdin cannot be read.
	closed, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	closed.Close()
	if canAnswer(closed) {
		t.Error("a closed stdin must not be treated as answerable")
	}
}
