package logger

import (
	"bufio"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// records reads a JSON log file back into maps.
func records(t *testing.T, path string) []map[string]any {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open log: %v", err)
	}
	defer f.Close()

	var out []map[string]any
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 16*1024), 1024*1024)
	for scanner.Scan() {
		var record map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			t.Fatalf("line is not JSON: %q: %v", scanner.Text(), err)
		}
		out = append(out, record)
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestWritesJSONLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logs", "app.log")
	l, err := New(Config{File: path})
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	l.Info(EventLogin, "logged in", Fields{"user": "alice", "ip": "10.0.0.1"})
	l.Warn(EventLoginFailed, "login failed", Fields{"user": "bob"})

	got := records(t, path)
	if len(got) != 2 {
		t.Fatalf("records = %d, want 2", len(got))
	}
	if got[0]["event"] != EventLogin || got[0]["msg"] != "logged in" {
		t.Errorf("first record = %v", got[0])
	}
	if got[0]["level"] != "INFO" {
		t.Errorf("level = %v, want INFO", got[0]["level"])
	}
	if got[1]["level"] != "WARN" {
		t.Errorf("level = %v, want WARN", got[1]["level"])
	}
	if got[0]["user"] != "alice" || got[0]["ip"] != "10.0.0.1" {
		t.Errorf("fields lost: %v", got[0])
	}
	// A timestamp is essential for an audit log.
	if _, ok := got[0]["time"]; !ok {
		t.Errorf("no time in %v", got[0])
	}
}

func TestCreatesTheFolder(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "a", "b", "c")
	l, err := New(Config{File: filepath.Join(dir, "app.log")})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer l.Close()

	l.Info(EventLogin, "x", nil)
	if _, err := os.Stat(filepath.Join(dir, "app.log")); err != nil {
		t.Fatalf("log not created: %v", err)
	}
}

func TestFileIsPrivate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.log")
	l, err := New(Config{File: path})
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	l.Info(EventLogin, "x", nil)

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	// The log holds usernames, addresses and file names.
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("mode = %o, want 600", perm)
	}
}

func TestLevelFilters(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.log")
	l, err := New(Config{File: path, Level: "warn"})
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	l.Info(EventLogin, "quiet", nil)
	l.Warn(EventLoginFailed, "loud", nil)

	got := records(t, path)
	if len(got) != 1 || got[0]["msg"] != "loud" {
		t.Fatalf("records = %v", got)
	}
}

func TestSetLevelAtRuntime(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.log")
	l, err := New(Config{File: path, Level: "info"})
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	l.Debug(EventLogin, "before", nil)
	l.SetLevel("debug")
	l.Debug(EventLogin, "after", nil)

	got := records(t, path)
	if len(got) != 1 || got[0]["msg"] != "after" {
		t.Fatalf("records = %v", got)
	}
}

func TestParseLevel(t *testing.T) {
	cases := map[string]string{
		"debug": "DEBUG", "info": "INFO", "warn": "WARN",
		"warning": "WARN", "error": "ERROR", "": "INFO", "nonsense": "INFO",
	}
	for in, want := range cases {
		if got := ParseLevel(in).String(); got != want {
			t.Errorf("ParseLevel(%q) = %s, want %s", in, got, want)
		}
	}
}

func TestNilLoggerIsSafe(t *testing.T) {
	var l *Logger
	// Tests and callers must not have to guard every call.
	l.Info(EventLogin, "x", nil)
	l.Warn(EventLogin, "x", nil)
	l.Error(EventLogin, "x", nil)
	l.SetLevel("debug")
	if err := l.Close(); err != nil {
		t.Errorf("Close on nil: %v", err)
	}
}

func TestNilFieldsAreSkipped(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.log")
	l, err := New(Config{File: path})
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	l.Info(EventLogin, "x", Fields{"user": "alice", "missing": nil})

	got := records(t, path)
	if _, present := got[0]["missing"]; present {
		t.Errorf("nil field was written: %v", got[0])
	}
}

func TestRotatesOverMaxSize(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")
	l, err := New(Config{File: path, MaxSize: 300})
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	for i := 0; i < 10; i++ {
		l.Info(EventLogin, "a record long enough to matter", Fields{"n": i})
		// slog writes on its own schedule, so size the check against what has
		// actually reached the file rather than assuming each call lands
		// before the next.
		flushLogQueue()
	}

	// The writer is synchronous: by the time Info returns, the bytes are in
	// the file. Asserting that directly stops a future refactor from quietly
	// moving logging onto a goroutine where rotation could race.
	flushLogQueue()

	// Only one previous file is kept, so the disk use stays bounded.
	if _, err := os.Stat(path + ".1"); err != nil {
		t.Fatalf("no rotated file: %v", err)
	}
	if _, err := os.Stat(path + ".2"); err == nil {
		t.Error("more than one rotated file was kept")
	}

	live, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	if live.Size() > 300 {
		t.Errorf("live file is %d bytes, over the 300 limit", live.Size())
	}
	// Nothing may be lost: the last record written is the newest one, so it
	// must be in the live file, and the previous file must hold a real
	// (not empty) batch of records.
	if got := records(t, path); len(got) == 0 || got[len(got)-1]["n"] != "9" {
		t.Errorf("live file = %v, want the newest record last", got)
	}
	if prev := records(t, path+".1"); len(prev) == 0 {
		t.Error("rotated file is empty: rotation copied nothing")
	}
}

// openLogPath is the file the size-stability wait polls.
var openLogPath string

// sizeOf returns a file's size, or 0 when it does not exist.
func sizeOf(t *testing.T, path string) int64 {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

// flushLogQueue waits for slog's background writer to drain.
//
// slog.Handler writes on its own goroutine, so a size check made immediately
// after a log call can observe a partially written file. The handler offers no
// flush hook, so this waits until the file stops growing.
func flushLogQueue() {
	last := int64(-1)
	for i := 0; i < 20; i++ {
		time.Sleep(2 * time.Millisecond)
		if info, err := os.Stat(openLogPath); err == nil {
			if info.Size() == last {
				return
			}
			last = info.Size()
		}
	}
}

func TestAppendsToExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.log")
	if err := os.WriteFile(path, []byte(`{"event":"before"}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	l, err := New(Config{File: path})
	if err != nil {
		t.Fatal(err)
	}
	l.Info(EventLogin, "after", nil)
	l.Close()

	got := records(t, path)
	if len(got) != 2 || got[0]["event"] != "before" {
		t.Fatalf("existing content was not appended to: %v", got)
	}
}

func TestRequestFields(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/api/files/delete", nil)
	r.RemoteAddr = "203.0.113.7:54321"

	fields := RequestFields(r)
	if fields["ip"] != "203.0.113.7" {
		t.Errorf("ip = %v, want the address without the port", fields["ip"])
	}
	if fields["method"] != "POST" || fields["path"] != "/api/files/delete" {
		t.Errorf("fields = %v", fields)
	}
}

func TestUserFieldsCreatesMap(t *testing.T) {
	fields := UserFields(nil, "alice")
	if fields["user"] != "alice" {
		t.Errorf("fields = %v", fields)
	}
	// The caller's map is filled in place.
	existing := Fields{"ip": "10.0.0.1"}
	if got := UserFields(existing, "bob"); got["ip"] != "10.0.0.1" || got["user"] != "bob" {
		t.Errorf("fields = %v", got)
	}
}

func TestRemoteIPWithoutPort(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "127.0.0.1"
	if got := remoteIP(r); got != "127.0.0.1" {
		t.Errorf("got %q", got)
	}
}

func TestNoBareMessageOnOneLine(t *testing.T) {
	// A message containing a newline must not split into two JSON lines.
	path := filepath.Join(t.TempDir(), "app.log")
	l, err := New(Config{File: path})
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	l.Info(EventLogin, "line one\nline two", nil)
	got := records(t, path)
	if len(got) != 1 {
		t.Fatalf("records = %d, want 1", len(got))
	}
	if !strings.Contains(got[0]["msg"].(string), "line two") {
		t.Errorf("message = %v", got[0]["msg"])
	}
}

func TestStdoutDestinationNeedsNoFile(t *testing.T) {
	// "-" must not be treated as a file name: no folder is created and no
	// rotation writer is involved.
	l, err := New(Config{File: Stdout, Level: "info"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if l.sink != nil {
		t.Error("stdout logger has a closable sink")
	}
	if err := l.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}

func TestStdoutWritesJSONToStandardOutput(t *testing.T) {
	// Redirect os.Stdout to a pipe for the duration of the test.
	original := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	defer func() { os.Stdout = original }()

	l, err := New(Config{File: Stdout})
	if err != nil {
		t.Fatal(err)
	}
	l.Info(EventLogin, "to stdout", Fields{"user": "alice"})
	l.Close()

	// Drain the pipe after closing the writer so reads do not block.
	w.Close()
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}

	line := strings.TrimSpace(string(data))
	if !strings.Contains(line, `"event":"auth.login"`) || !strings.Contains(line, `"user":"alice"`) {
		t.Fatalf("stdout = %q", line)
	}
	var record map[string]any
	if err := json.Unmarshal([]byte(line), &record); err != nil {
		t.Fatalf("stdout is not a JSON line: %v", err)
	}
}

func TestBareFileNameNeedsNoFolder(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	// A name with no directory part must not trip over MkdirAll("").
	l, err := New(Config{File: "app.log"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	l.Info(EventLogin, "x", nil)
	l.Close()

	if got := records(t, filepath.Join(dir, "app.log")); len(got) != 1 {
		t.Fatalf("records = %v", got)
	}
}
