package app

import (
	"strings"
	"testing"
)

// TestPhoneRowsShowSizeAndDate keeps the compact metadata line from
// disappearing again: on a phone the name row alone hid the size and date, and
// the menu does not carry them.
func TestPhoneRowsShowSizeAndDate(t *testing.T) {
	css := readAsset(t, "css/browser.css")

	rules := ruleIndex(css, ".file-table .col-meta")
	if len(rules) == 0 {
		t.Fatal("browser.css has no .file-table .col-meta rule")
	}
	last := rules[len(rules)-1]
	block := ruleBlock(css, last)
	for _, want := range []string{"display: block", "grid-area: meta"} {
		if !strings.Contains(block, want) {
			t.Errorf("the phone .col-meta rule is missing %q", want)
		}
	}
	if strings.LastIndex(css[:last], "@media (max-width: 560px)") < 0 {
		t.Error("the visible .col-meta rule is not inside the phone media query")
	}
}
