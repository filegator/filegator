package app

import (
	"strings"
	"testing"
)

// TestGalleryHasBottomThumbnailsAndFileActions pins the requested layout: the
// thumbnail strip is below the image again, and the gallery header offers the
// same file actions as a row so an image can be moved or deleted without
// closing it first.
func TestGalleryHasBottomThumbnailsAndFileActions(t *testing.T) {
	css := readAsset(t, "css/dialog.css")

	rules := ruleIndex(css, ".gallery-strip")
	if len(rules) == 0 {
		t.Fatal("dialog.css has no .gallery-strip rule")
	}
	strip := ruleBlock(css, rules[len(rules)-1])
	for _, want := range []string{"overflow-x: auto", "overflow-y: hidden"} {
		if !strings.Contains(strip, want) {
			t.Errorf("the gallery thumbnails are not a horizontal bottom strip: missing %q", want)
		}
	}
	if strings.Contains(strip, "flex-direction: column") {
		t.Error("the gallery thumbnails are still a vertical column")
	}

	galleryRules := ruleIndex(css, ".gallery")
	hasSurface := false
	for _, rule := range galleryRules {
		if strings.Contains(ruleBlock(css, rule), "background: var(--surface)") {
			hasSurface = true
			break
		}
	}
	if !hasSurface {
		t.Error("the gallery has no surface behind its title")
	}
	if !strings.Contains(css, ".gallery > .dialog-header") || !strings.Contains(css, "align-items: center") {
		t.Error("the gallery counter is not centered with the close button")
	}

	js := readAsset(t, "js/views/gallery.js")
	if !strings.Contains(js, "scrollThumbIntoView") || !strings.Contains(js, "requestAnimationFrame") {
		t.Error("gallery.js does not animate the thumbnail strip when the image changes")
	}
	if !strings.Contains(js, "prefers-reduced-motion") {
		t.Error("the thumbnail animation ignores reduced-motion preferences")
	}
	thumbRules := ruleIndex(css, ".gallery-thumb")
	if len(thumbRules) == 0 {
		t.Fatal("dialog.css has no .gallery-thumb rule")
	}
	hasTransition := false
	for _, rule := range thumbRules {
		if strings.Contains(ruleBlock(css, rule), "transition: opacity 120ms ease") {
			hasTransition = true
			break
		}
	}
	if !hasTransition {
		t.Error("the active thumbnail has no short transition")
	}
	if !strings.Contains(js, "openRowMenu") || !strings.Contains(js, "options.onRowAction") {
		t.Error("the gallery does not offer the row actions menu")
	}
	if !strings.Contains(js, "elevated: true") {
		t.Error("the gallery menu would render behind the gallery dialog")
	}
	if !strings.Contains(js, `className: "gallery-footer-actions"`) ||
		!strings.Contains(js, `className: "gallery-caption"`) ||
		strings.Contains(js, "gallery-header-actions") {
		t.Error("the image name and action menu should be in the footer")
	}
	if strings.Contains(js, "gallery-download") {
		t.Error("the standalone Download button should not be rendered")
	}
	if strings.Contains(js, "gallery-status") || strings.Contains(css, ".gallery-status") {
		t.Error("the gallery footer still shows the image size")
	}
	if !strings.Contains(css, ".gallery-caption") || !strings.Contains(css, "align-items: center") {
		t.Error("the image name is not aligned with the footer controls")
	}
	if !strings.Contains(js, `action.id !== "view"`) {
		t.Error("the gallery menu duplicates the open image with a View action")
	}
	if strings.Contains(js, `action.id !== "download"`) {
		t.Error("Download should also be available in the gallery actions menu")
	}

	browser := readAsset(t, "js/views/browser.js")
	if !strings.Contains(browser, "rowActions: (item) => rowActions") ||
		!strings.Contains(browser, "onRowAction: (id, item, specs) => runRowAction") {
		t.Error("browser.js does not pass the row actions into the gallery")
	}
}
