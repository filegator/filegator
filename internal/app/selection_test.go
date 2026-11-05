package app

import (
	"strings"
	"testing"
)

// TestShiftClickSelectsARange guards desktop range selection: without the
// anchor and shift handling, every click toggled one box and a folder of files
// had to be selected one at a time.
func TestShiftClickSelectsARange(t *testing.T) {
	src := readAsset(t, "js/views/browser.js")

	for _, want := range []string{
		"let selectionAnchor = null",
		`checkbox.addEventListener("mousedown"`,
		"event.shiftKey",
		"selectRangeTo(entry.path, checkbox.checked)",
		"querySelectorAll(\".row-check[data-path]\")",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("browser.js is missing %q", want)
		}
	}
	// Selecting everything starts a new range, not an old one.
	if !strings.Contains(src, "selectionAnchor = null;\n    if (selectAll.checked)") {
		t.Error("select-all should reset the shift-click anchor")
	}
}
