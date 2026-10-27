package app

import (
	"regexp"
	"strings"
	"testing"
)

// The theme module's branches are asserted here rather than by running the
// JavaScript: the project has no Node and no browser automation, and these are
// the branches that matter — a stored choice beating the system, a broken
// storage API not breaking boot, and the attribute being removed (not set to a
// third value) when the user follows the system.

// TestThemeModuleHasTheBranches checks that each rule is written in the module
// at all. A rewrite that drops one of them fails here rather than showing up
// as "the toggle does nothing on Firefox".
func TestThemeModuleHasTheBranches(t *testing.T) {
	src := readAsset(t, "js/modules/theme.js")

	wants := map[string]string{
		"a stored choice wins over the system":          `if (stored === LIGHT || stored === DARK) {`,
		"a broken storage does not break boot":          `} catch (e) {`,
		"the system is read through matchMedia":         `query.matches ? DARK : LIGHT`,
		"following the system removes the attribute":    `root.removeAttribute("data-theme")`,
		"a choice is remembered":                        `storage.setItem(STORAGE_KEY, theme)`,
		"the system is followed while the page is open": `query.addEventListener("change"`,
	}
	for reason, want := range wants {
		if !strings.Contains(src, want) {
			t.Errorf("theme.js does not handle %s (missing %q)", reason, want)
		}
	}

	// The attribute is either absent or one of the two themes. A third value
	// such as "system" would be selected by neither block in the stylesheet,
	// so the page would fall back to light on a dark machine.
	if regexp.MustCompile(`setAttribute\("data-theme",\s*"([^"]*)"`).MatchString(src) {
		t.Error("theme.js hardcodes a data-theme value; it must set only LIGHT or DARK")
	}
	if !strings.Contains(src, `export const LIGHT = "light"`) || !strings.Contains(src, `export const DARK = "dark"`) {
		t.Error("theme.js must export LIGHT and DARK for app.js and the tests")
	}
}

// TestThemeStorageKeyMatchesTheInlineScript is the one duplication in the
// feature: index.html reads the key before any module loads, so the literal
// appears there as well as in theme.js. A rename in one place and not the other
// means the stored choice is never applied before the paint — the flash comes
// back, and nothing else would notice.
func TestThemeStorageKeyMatchesTheInlineScript(t *testing.T) {
	module := readAsset(t, "js/modules/theme.js")
	key := regexp.MustCompile(`STORAGE_KEY = "([^"]+)"`).FindStringSubmatch(module)
	if key == nil {
		t.Fatal("theme.js does not declare STORAGE_KEY")
	}

	page := readAsset(t, "index.html")
	if !strings.Contains(page, `localStorage.getItem("`+key[1]+`")`) {
		t.Errorf("index.html reads a different key than theme.js (%q); the stored theme would not apply before the paint", key[1])
	}
	// The inline script must accept only the two themes: a stale or hand-edited
	// value would otherwise become a data-theme the stylesheet does not know.
	if !strings.Contains(page, `savedTheme === "light" || savedTheme === "dark"`) {
		t.Error("the inline script should ignore any stored value that is not light or dark")
	}
}

// TestThemeMenuItemIsTranslated checks the menu's labels go through t(), so
// they follow the interface language like every other label. The theme item is
// built when the menu opens, which is why its label is not in app.js any more.
func TestThemeMenuItemIsTranslated(t *testing.T) {
	module := readAsset(t, "js/modules/user_menu.js")
	if !strings.Contains(module, "t(action.label)") && !strings.Contains(module, "t(item.label)") {
		t.Error("user_menu.js does not translate its item labels")
	}
	// The theme item's label is chosen from a translation key, not a literal.
	if !strings.Contains(module, `"Dark mode"`) || !strings.Contains(module, `"Light mode"`) {
		t.Error("the theme item does not use the Dark mode / Light mode keys")
	}
	// And the button that opens the menu is labelled as an account menu.
	if !strings.Contains(readAsset(t, "js/views/browser.js"), `t("Account")`) {
		t.Error("the account button has no translated label")
	}
}
