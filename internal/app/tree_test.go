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
