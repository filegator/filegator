package app

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/filegator/filegator/internal/config"
)

// startupNotesSample is a users.json template shaped like the shipped one, so
// a first start copies it instead of falling back to the built-in accounts.
const startupNotesSample = `{"1":{"username":"admin","name":"Admin","role":"admin","homedir":"/","permissions":"read|write","password":"$2y$10$Nu35w4pteLfc7BDCIkDPkecjw8wsH8Y2GMfIewUbXLT7zzW6WOxwq"}}`

// firstStart starts an application in a folder that has never run before: no
// configuration file, no users file, no storage folder.
//
// Building the stores writes a configuration file, so it is removed just
// before New() runs. That is exactly the state of a binary started in an empty
// directory, which is the case these notes exist for.
func firstStart(t *testing.T, dir string, withSample bool) *App {
	t.Helper()
	if withSample {
		sample := filepath.Join(dir, "private", "users.json.sample")
		if err := os.MkdirAll(filepath.Dir(sample), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(sample, []byte(startupNotesSample), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	path := filepath.Join(dir, "config.json")
	store, err := config.Load(path)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if err := store.Update(func(c *config.Config) error {
		c.Storage.Root = filepath.Join(dir, "repository")
		c.Auth.UsersFile = filepath.Join(dir, "private", "users.json")
		return nil
	}); err != nil {
		t.Fatalf("config.Update: %v", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatalf("remove config: %v", err)
	}

	a, err := New(store, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { a.Close() })
	return a
}

// secondStart starts an application over a folder firstStart already ran in.
func secondStart(t *testing.T, dir string) *App {
	t.Helper()
	store, err := config.Load(filepath.Join(dir, "config.json"))
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	a, err := New(store, "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { a.Close() })
	return a
}

// noteKeys returns each note's key and kind.
func noteKeys(notes []StartupNote) map[string]string {
	out := map[string]string{}
	for _, note := range notes {
		out[note.Key] = note.Kind
	}
	return out
}

func hasNote(notes []StartupNote, key string) bool {
	_, ok := noteKeys(notes)[key]
	return ok
}

// TestStartupNotesRecordAFreshStart checks that a binary started in an empty
// folder tells the administrator what it created: the configuration file with
// a new session key, the storage folder, and the users file copied from the
// sample. The startup log says the same, but nobody is watching the log on a
// first start; the interface is where the decision to keep or fix it is made.
func TestStartupNotesRecordAFreshStart(t *testing.T) {
	dir := t.TempDir()
	a := firstStart(t, dir, true)

	notes := a.startup.Notes()
	keys := noteKeys(notes)
	for _, want := range []string{"config.created", "storage.created", "users.created"} {
		if _, ok := keys[want]; !ok {
			t.Errorf("no %q note; got %v", want, keys)
		}
		if keys[want] != noteInfo {
			t.Errorf("%s kind = %q, want %q", want, keys[want], noteInfo)
		}
	}
	// "created" supersedes "missing": showing both would read as a bug.
	if hasNote(notes, "config.missing") {
		t.Error("a missing config that was then created should not still be reported as missing")
	}
	if _, err := os.Stat(filepath.Join(dir, "config.json")); err != nil {
		t.Error("the configuration file should have been created on a writable first start")
	}
}

// TestStartupNotesWarnAboutAMissingUsersFile covers the case an operator most
// needs told about: no users file and no sample to copy it from, so nobody
// exists on disk and the built-in accounts are all there is.
func TestStartupNotesWarnAboutAMissingUsersFile(t *testing.T) {
	a := firstStart(t, t.TempDir(), false)
	keys := noteKeys(a.startup.Notes())
	if keys["users.builtin"] != noteWarning {
		t.Fatalf("users.builtin = %q, want a %q note; got %v", keys["users.builtin"], noteWarning, keys)
	}
}

// TestStartupNotesSilentOnASecondStart keeps them useful: an instance that has
// been running for months must not carry a wall of "created" lines.
func TestStartupNotesSilentOnASecondStart(t *testing.T) {
	dir := t.TempDir()
	firstStart(t, dir, true)
	second := secondStart(t, dir)

	if notes := second.startup.Notes(); len(notes) > 0 {
		t.Errorf("a warm start produced notes: %v", notes)
	}
}

// TestStartupNotesReachTheBrowserOverPublicConfig checks the delivery path: the
// page already fetches /api/config, so the notes travel with it.
func TestStartupNotesReachTheBrowserOverPublicConfig(t *testing.T) {
	a := firstStart(t, t.TempDir(), true)
	rec := get(a, "/api/config")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var envelope struct {
		Data struct {
			StartupNotes []StartupNote `json:"startup_notes"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if len(envelope.Data.StartupNotes) == 0 {
		t.Fatal("the public config should carry the startup notes")
	}
	// The users file is configurable and must never be named there: the note
	// mentions only the default name.
	if strings.Contains(rec.Body.String(), "users.json.sample") {
		t.Errorf("a configured users path leaked into the public config: %s", rec.Body)
	}
}

// TestFirstStartWritesTheConfigOnTheFirstRun is the sequence an operator
// reported: start a binary in an empty folder, save a setting, restart. The
// first start used to be read-only ("cannot be saved") because private/ did not
// exist yet, and only the second start wrote config.json — because the users
// provider had created private/ in between.
func TestFirstStartWritesTheConfigOnTheFirstRun(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	a := firstStart(t, dir, true)
	if a.config.ReadOnly() {
		t.Fatalf("the first start is read-only: %s", a.config.Why())
	}
	if hasNote(a.startup.Notes(), "config.read_only") || hasNote(a.startup.Notes(), "config.no_session_key") {
		t.Errorf("the first start reported a read-only configuration: %v", a.startup.Notes())
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("config.json should exist after the first start: %v", err)
	}
	a.Close()

	// Saving a setting works without a restart, and survives one.
	if err := a.config.Update(func(c *config.Config) error {
		c.Frontend.AppName = "Renamed"
		return nil
	}); err != nil {
		t.Fatalf("saving on the first start: %v", err)
	}
	reloaded, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := reloaded.Get().Frontend.AppName; got != "Renamed" {
		t.Errorf("app name after reload = %q", got)
	}
}
