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

// TestPhoneHeaderKeepsSelectAllAndSorting guards the controls above the list:
// hiding the whole header removed select-all and every sort button on a phone.
func TestPhoneHeaderKeepsSelectAllAndSorting(t *testing.T) {
	css := readAsset(t, "css/browser.css")

	rules := ruleIndex(css, ".file-table thead")
	if len(rules) == 0 {
		t.Fatal("browser.css has no .file-table thead rule")
	}
	last := rules[len(rules)-1]
	if !strings.Contains(ruleBlock(css, last), "display: block") {
		t.Error("the phone listing hides the header row")
	}
	if strings.LastIndex(css[:last], "@media (max-width: 560px)") < 0 {
		t.Error("the visible header rule is not inside the phone media query")
	}
	// Only the body cells lose the desktop columns; the header keeps its sort
	// buttons.
	if !strings.Contains(css, ".file-table tbody .col-size") ||
		!strings.Contains(css, ".file-table tbody .col-time") {
		t.Error("the phone rules hide the header sort buttons along with the body columns")
	}
}

// TestPhoneSortIsOneCompactControl checks the three desktop sort buttons
// collapse into one small control on a phone, and picking the active column
// again flips the direction the way a desktop header click does.
func TestPhoneSortIsOneCompactControl(t *testing.T) {
	css := readAsset(t, "css/browser.css")
	page := readAsset(t, "index.html")
	js := readAsset(t, "js/views/browser.js")

	if !strings.Contains(page, `id="sort-mobile"`) || !strings.Contains(page, "sort-mobile-button") {
		t.Error("index.html has no mobile sort control")
	}
	if strings.Contains(page, `<select id="sort-mobile"`) {
		t.Error("the mobile sort control is still a select")
	}
	if !strings.Contains(css, ".sort-mobile-button") || !strings.Contains(css, ".file-table thead th.col-sort-mobile") {
		t.Error("the mobile sort control has no styles")
	}
	if !strings.Contains(js, `sortMobile.addEventListener("click"`) ||
		!strings.Contains(js, "onChoose: (id) => setSort(id)") {
		t.Error("browser.js does not open the mobile sort choices")
	}
	if !strings.Contains(js, "nextDirection(key, sortKey, sortDesc)") {
		t.Error("choosing the active sort column no longer toggles the direction")
	}
	if !strings.Contains(js, `sortDesc ? "▼" : "▲"`) {
		t.Error("the mobile sort arrow does not follow the sort direction")
	}
	if strings.Contains(js, "SORT_LABELS[sortKey]} ▾") {
		t.Error("the mobile sort button should show only the direction arrow")
	}
	rules := ruleIndex(css, ".file-table thead th.col-sort-mobile")
	if len(rules) == 0 || !strings.Contains(ruleBlock(css, rules[len(rules)-1]), "margin-left: auto") {
		t.Error("the mobile sort control is not on the right")
	}
}
