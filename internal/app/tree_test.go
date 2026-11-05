package app

import (
	"strings"
	"testing"
)

// TestTreeWaitsForAnInFlightLoad guards the first-click bug: the sidebar can
// still be loading a folder when its arrow is clicked, and a click used to
// collapse the half-opened node. Waiting on the shared load keeps the first
// click from being lost.
func TestTreeWaitsForAnInFlightLoad(t *testing.T) {
	src := readAsset(t, "js/modules/tree.js")

	for _, want := range []string{
		"this.pending = null",
		"if (node.loading) {",
		"await node.pending;",
		"node.pending = (async () => {",
		"node.pending = null;",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("tree.js is missing %q", want)
		}
	}
	if strings.Contains(src, "node.expanded = false;\n\t}\n\treturn toggleNode(node);") {
		t.Error("expandNode still flips an expanded node closed before loading it")
	}
}

// TestTreeOffersRightClickActions checks a folder in the sidebar can open the
// same menu as its file-list row.
func TestTreeOffersRightClickActions(t *testing.T) {
	tree := readAsset(t, "js/modules/tree.js")
	browser := readAsset(t, "js/views/browser.js")

	for _, want := range []string{"onContextMenu", `"contextmenu"`} {
		if !strings.Contains(tree, want) {
			t.Errorf("tree.js is missing %q", want)
		}
	}
	for _, want := range []string{
		"onContextMenu: (node, event, anchor) => {",
		"rowActions({ entry, can, isEditable, isImage })",
		"openRowMenu({",
		"onChoose: (id) => runRowAction(id, entry, specs)",
	} {
		if !strings.Contains(browser, want) {
			t.Errorf("browser.js is missing %q", want)
		}
	}
}

// TestTreeReloadsAfterMutations guards the stale-path bug: a moved folder
// remained in the cached tree and clicking it produced "path not found".
func TestTreeReloadsAfterMutations(t *testing.T) {
	tree := readAsset(t, "js/modules/tree.js")
	browser := readAsset(t, "js/views/browser.js")

	for _, want := range []string{"async function reload()", "reload,", "collectExpanded", "reloadNode"} {
		if !strings.Contains(tree, want) {
			t.Errorf("tree.js is missing %q", want)
		}
	}
	if !strings.Contains(browser, "async function refreshTreeAfterMutation()") {
		t.Fatal("browser.js has no tree refresh after mutations")
	}
	if strings.Count(browser, "await refreshTreeAfterMutation();") < 5 {
		t.Error("not every mutating path refreshes the tree")
	}
}
