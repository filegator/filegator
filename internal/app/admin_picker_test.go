package app

import (
	"io/fs"
	"strings"
	"testing"
)

// TestHomePickerFallsBackWhenStoredHomeIsGone guards a lockout the picker
// caused: deleting a user's home folder left the picker starting at a path the
// server answers 404 for, so its list stayed empty and the administrator could
// not navigate to another folder. The folder had to be recreated first. The
// first load now climbs to the nearest ancestor that still exists, which gives
// the administrator a way back to the storage root.
func TestHomePickerFallsBackWhenStoredHomeIsGone(t *testing.T) {
	data, err := fs.ReadFile(frontendRoot(), "js/views/admin.js")
	if err != nil {
		t.Fatal(err)
	}
	body := adminFunction(t, string(data), "chooseRoot")

	for _, want := range []string{
		`render(path, true);`,
		`if (fallback && !atTop(dir)) {`,
		`return render(parentOf(dir), true);`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("chooseRoot() is missing %q", want)
		}
	}
}
