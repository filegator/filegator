package app

import (
	"regexp"
	"strings"
	"testing"
)

// Reported on a phone: open search, use a result's ⋯ menu, choose Move — and
// the folder picker appeared behind the search results.
//
// Two halves to the fix, and both are pinned here:
//
//  1. An action that opens a dialog closes the dropdown first. A modal over an
//     open dropdown leaves the dropdown painted under its backdrop, so the
//     reader sees results they cannot dismiss.
//  2. The dialog layer sits above every transient popover anyway
//     (z-index 100), so nothing else can be found behind a modal either.

// TestActionsThatOpenADialogCloseTheResults checks the list is the one the
// dialogs actually use, action by action.
func TestActionsThatOpenADialogCloseTheResults(t *testing.T) {
	src := readAsset(t, "js/modules/header_search.js")

	block := src[strings.Index(src, "const ACTIONS_OPEN_A_DIALOG"):]
	block = block[:strings.Index(block, "]);")]

	// Every action that opens something modal. `edit` and `view` open the
	// editor and the gallery, which are full-screen overlays, not dialogs, and
	// are excluded for that reason below.
	for _, action := range []string{"copy", "move", "rename", "zip", "chmod", "delete"} {
		if !strings.Contains(block, `"`+action+`"`) {
			t.Errorf("%s opens a dialog but does not close the results first", action)
		}
	}
	// Download and unzip work in place: the remaining results stay usable,
	// which is the documented behaviour and must not regress.
	for _, action := range []string{"download", "unzip"} {
		if strings.Contains(block, `"`+action+`"`) {
			t.Errorf("%s acts in place, so it should leave the results open", action)
		}
	}

	// And the close actually happens before the action runs.
	closeAt := strings.Index(src, "ACTIONS_OPEN_A_DIALOG.has(id)")
	actionAt := strings.Index(src, "onAction(id, result)")
	if closeAt < 0 || actionAt < 0 {
		t.Fatal("the onChoose handler no longer closes the results or calls onAction")
	}
	if closeAt > actionAt {
		t.Error("the results are closed after the action runs; a dialog may already be open by then")
	}
}

// TestDialogsSitAboveEveryTransientLayer is the second half: whatever opens a
// dialog, the dialog must be on top of the menus, the dropdown, the upload
// panel and the notices.
func TestDialogsSitAboveEveryTransientLayer(t *testing.T) {
	dialog := readAsset(t, "css/dialog.css")
	at := strings.Index(dialog, ".dialog-root {")
	if at < 0 {
		t.Fatal("no .dialog-root rule")
	}
	rule := dialog[at : at+strings.Index(dialog[at:], "}")]
	value := regexp.MustCompile(`z-index:\s*(\d+)`).FindStringSubmatch(rule)
	if value == nil {
		t.Fatal(".dialog-root has no z-index")
	}
	if value[1] != "100" {
		t.Errorf(".dialog-root z-index = %s, want 100; see the stacking comment in dialog.css", value[1])
	}

	// Every other layer must be below it. These are the numbers the other
	// stylesheets use; if one of them rises above 100, a dialog could be found
	// behind it again.
	for file, want := range map[string]int{
		"css/browser.css": 90, // tree drawer, the highest non-dialog layer
		"css/editor.css":  80,
		"css/notices.css": 60,
	} {
		css := readAsset(t, file)
		for _, m := range regexp.MustCompile(`z-index:\s*(\d+)`).FindAllStringSubmatch(css, -1) {
			if n := atoi(m[1]); n > want {
				t.Errorf("%s uses z-index %d, above the expected ceiling of %d", file, n, want)
			}
		}
	}
}

func atoi(s string) int {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0
		}
		n = n*10 + int(r-'0')
	}
	return n
}
