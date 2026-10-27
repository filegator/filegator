package app

import (
	"io/fs"
	"os"
	"strings"
	"testing"
)

// TestStartupNotesAreShownToAdministrators guards the path from the server to
// the notice bar: the notes are in /api/config, app.js keeps them, and the
// notice bar renders them only for an administrator (it asks the admin-only
// status endpoint first, and a non-admin never reaches the render).
func TestStartupNotesAreShownToAdministrators(t *testing.T) {
	read := func(name string) string {
		t.Helper()
		data, err := fs.ReadFile(frontendRoot(), name)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}

	app := read("js/app.js")
	for _, want := range []string{"let startupNotes = [];", "startupNotes = config.startup_notes || [];", "startup: () => startupNotes,"} {
		if !strings.Contains(app, want) {
			t.Errorf("app.js should carry the startup notes to the notice bar (missing %q)", want)
		}
	}

	notices := read("js/modules/notices.js")
	if !strings.Contains(notices, "export function startupNotices(notes)") {
		t.Fatal("notices.js should expose startupNotices()")
	}
	// Start-up notices first: they describe the server, the rest describe
	// settings the administrator can change.
	if !strings.Contains(notices, "current = [...startupNotices(startNotes), ...noticesFor(status)];") {
		t.Error("the notice bar should render the startup notes before the status notices")
	}
	// They come back from an admin-only request, so a regular user sees
	// nothing: the render happens after api.adminStatus() succeeded.
	status := strings.Index(notices, "await api.adminStatus()")
	render := strings.Index(notices, "startupNotices(startNotes)")
	if status < 0 || render < 0 || render < status {
		t.Error("startup notes must only be rendered for administrators, after the status request succeeds")
	}
}

// TestStartupNotesKeysAreStable pins the keys clients could act on, so a rename
// is a deliberate change rather than a typo in a string concatenation. The
// keys live next to the code that raises them, so the assertion covers both
// files.
func TestStartupNotesKeysAreStable(t *testing.T) {
	var source strings.Builder
	for _, name := range []string{"startup_notes.go", "app.go", "auth.go", "storage.go"} {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		source.Write(data)
	}
	for _, key := range []string{
		`"config.missing"`, `"config.created"`, `"config.no_session_key"`,
		`"config.read_only"`, `"config.unreadable"`,
		`"storage.created"`, `"users.created"`, `"users.builtin"`, `"users.unwritable"`,
	} {
		if !strings.Contains(source.String(), key) {
			t.Errorf("no startup note uses the key %s any more", key)
		}
	}
}
