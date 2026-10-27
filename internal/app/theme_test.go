package app

import (
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The theme tests are assertions about the served files, not a browser run:
// the project has no browser automation. They pin the things that would break
// silently — a palette that is missing a surface, a toggle the page does not
// show, or a stored choice that paints late — rather than the exact colours,
// which are a design decision and can change without breaking anything.

// TestThemeIsAppliedBeforeTheFirstPaint checks the inline script in <head>.
//
// Without it the page paints light and turns dark once app.js runs, which is a
// visible flash on every navigation. The script must be inline (a separate
// file is a network round trip after the paint) and it must run before the
// stylesheets, so its position in <head> matters, not just its presence.
func TestThemeIsAppliedBeforeTheFirstPaint(t *testing.T) {
	rec := get(newTestApp(t), "/")
	body, _ := io.ReadAll(rec.Body)
	page := string(body)

	script := strings.Index(page, "localStorage.getItem(\"filegator.theme\")")
	if script < 0 {
		t.Fatal("the page does not read the stored theme in <head>")
	}
	link := strings.Index(page, `<link rel="stylesheet"`)
	if link < 0 {
		t.Fatal("the page has no stylesheets")
	}
	if script > link {
		t.Error("the theme script runs after the stylesheets, so the page flashes light")
	}
	// Any external script here would be requested after the first paint.
	// Two inline scripts are expected: the theme, and the logo preload that
	// has to know the theme to pick the right file. Both are inline for the
	// same reason, and both must come before the stylesheets.
	head := page[:strings.Index(page, "</head>")]
	inline := regexp.MustCompile(`<script(?:\s[^>]*)?>(?s:.+?)</script>`).FindAllString(head, -1)
	if len(inline) != 2 {
		t.Errorf("expected two inline scripts in <head> (theme, logo preload), found %d", len(inline))
	}
	for _, tag := range inline {
		if strings.Contains(tag, "src=") {
			t.Errorf("a <head> script is external (%s); it has to be inline", tag)
		}
		if at := strings.Index(head, tag); at > link {
			t.Error("a <head> script runs after the stylesheets, so the page flashes")
		}
	}
	// Both themes are declared, so the browser paints its own form controls,
	// scrollbars and the like for whichever is active.
	if !strings.Contains(page, `name="color-scheme" content="light dark"`) {
		t.Error("color-scheme should be \"light dark\" now that the interface has a dark theme")
	}
}

// TestTheThemeIsInTheAccountMenu pins where the light/dark choice lives.
//
// It is an item of the account menu (user decision 2026-10-27), not a toolbar
// button: it is a preference about the person, not about this listing, and it
// belongs with the password and signing out. The toolbar still swappes in
// place for a selection, and it no longer needs a theme control to hide.
func TestTheThemeIsInTheAccountMenu(t *testing.T) {
	page := readAsset(t, "index.html")

	if strings.Contains(page, `id="btn-theme"`) {
		t.Error("the old toolbar theme button is still in the page")
	}
	// The pathbar's browsing actions are Upload, New, Tree, Refresh only.
	toolbar := page[strings.Index(page, `class="pathbar-actions"`):]
	toolbar = toolbar[:strings.Index(toolbar, `id="selection-actions"`)]
	if strings.Contains(toolbar, "theme") {
		t.Error("the toolbar still carries a theme control")
	}

	// The menu exists, is anchored to its button, and starts hidden: it is
	// rendered by user_menu.js on every open, so nothing is hardcoded in it.
	button := regexp.MustCompile(`(?s)<button[^>]*id="btn-user-menu".*?</button>`).FindString(page)
	if button == "" {
		t.Fatal("no account menu button in the page")
	}
	for _, want := range []string{`aria-haspopup="menu"`, `aria-controls="user-menu"`, `aria-expanded="false"`} {
		if !strings.Contains(button, want) {
			t.Errorf("the account button is missing %s", want)
		}
	}
	if !strings.Contains(page, `id="user-menu"`) || !strings.Contains(page, `role="menu"`) {
		t.Error("no menu element for the account button")
	}

	// Setup stays a button of its own, next to the menu rather than inside it.
	actions := page[strings.Index(page, `<nav class="app-actions">`):]
	actions = actions[:strings.Index(actions, "</nav>")]
	admin := strings.Index(actions, `id="btn-admin"`)
	menu := strings.Index(actions, `id="btn-user-menu"`)
	if admin < 0 || menu < 0 || admin > menu {
		t.Error("Setup should be a button in the header, before the account menu")
	}
}

// TestTheAccountMenuItems pins which entries a session gets. The list is plain
// data so this is checked without a DOM.
func TestTheAccountMenuItems(t *testing.T) {
	module := readAsset(t, "js/modules/user_menu.js")
	for _, want := range []string{
		`label: theme === DARK ? "Light mode" : "Dark mode"`,
		`{ id: "password", label: "Change password"`,
		`items.push({ id: "login", label: "Log in"`,
		`{ id: "logout", label: "Logout"`,
		`if (!isGuest && canChangePassword !== false)`,
		`} else if (canLogout !== false) {`,
	} {
		if !strings.Contains(module, want) {
			t.Errorf("user_menu.js is missing %q", want)
		}
	}

	// The labels are translated when rendered, and every one of them exists.
	english := readAsset(t, "translations/english.json")
	for _, key := range []string{"Dark mode", "Light mode", "Change password", "Logout", "Log in", "Account"} {
		if !strings.Contains(english, `"`+key+`"`) {
			t.Errorf("english.json has no %q key; the menu item reads as English on every translated screen", key)
		}
	}
}

// TestTheLanguagePickerIsInTheMenu pins the language entry and the rules
// around it: everyone gets it (a guest reading a login form in an unknown
// language has nowhere else to switch), but only while the administrator
// allows a choice.
func TestTheLanguagePickerIsInTheMenu(t *testing.T) {
	menu := readAsset(t, "js/modules/user_menu.js")
	for _, want := range []string{
		`{ id: "language", label: "Language"`,
		`if (canPickLanguage) {`,
	} {
		if !strings.Contains(menu, want) {
			t.Errorf("user_menu.js is missing %q", want)
		}
	}
	// It is pushed before the guest/logout branch, so a guest gets it too.
	language := strings.Index(menu, `id: "language"`)
	guest := strings.Index(menu, `id: "login"`)
	if language < 0 || guest < 0 || language > guest {
		t.Error("the language item must come before the guest/logout branch, or a guest would not get it")
	}

	picker := readAsset(t, "js/modules/language_picker.js")
	for _, want := range []string{
		// The list and the gate come from the module that loaded them;
		// nothing here may invent its own.
		`languages`,
		`onChoose`,
		// A dismissed picker must not switch anything.
		`finish(null)`,
		// Both ways out, as every dialog has.
		`event.key === "Escape"`,
		`event.target === root`,
	} {
		if !strings.Contains(picker, want) {
			t.Errorf("language_picker.js is missing %q", want)
		}
	}

	// A native <select> in a dialog, not the radio list this first shipped with
	// (user decision 2026-10-27). The control carries the shared .select class,
	// so it looks like every other dropdown rather than a second design.
	for _, want := range []string{`create("select", {`, `className: "select"`, `option.selected = code === current`} {
		if !strings.Contains(picker, want) {
			t.Errorf("language_picker.js is missing %q", want)
		}
	}
	for _, gone := range []string{`type: "radio"`, `language-option`} {
		if strings.Contains(picker, gone) {
			t.Errorf("language_picker.js still builds a radio list (%q)", gone)
		}
	}

	// The label is the code with the first letter upcased (user decision
	// 2026-10-27): "german" -> "German", "portuguese_br" -> "Portuguese_br".
	for in, want := range map[string]string{
		"english":       "English",
		"portuguese_br": "Portuguese_br",
		"":              "",
	} {
		if got := languageLabelForTest(t, in); got != want {
			t.Errorf("languageLabel(%q) = %q, want %q", in, got, want)
		}
	}

	if !strings.Contains(readAsset(t, "translations/english.json"), `"Language"`) {
		t.Error("english.json has no Language key")
	}

	// The login screen shows a link that opens this same dialog, not a second
	// dropdown: a full-width select under the button doubled the card's height
	// for a setting a visitor changes once.
	login := readAsset(t, "js/views/login.js")
	if !strings.Contains(login, `el("login-language")`) {
		t.Error("login.js has no language link to open the picker")
	}
	if strings.Contains(login, `Object.assign(document.createElement("option")`) {
		t.Error("login.js still builds its own language options instead of opening the picker")
	}
	page := readAsset(t, "index.html")
	if strings.Index(page, `id="login-language"`) < 0 {
		t.Fatal("index.html has no #login-language element")
	}
	if strings.Contains(page, `<select id="login-language"`) {
		t.Error("index.html still renders a language <select> on the login card")
	}
}

// languageLabelForTest runs the module's label function under node, the way
// TestLogoModuleRunsWhatItClaims runs logo.js: the label is behaviour, and
// reading the source only proves a function exists.
func languageLabelForTest(t *testing.T, code string) string {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed; languageLabel is unchecked on this machine")
	}
	module, err := filepath.Abs(filepath.Join("frontend", "js", "modules", "language_picker.js"))
	if err != nil {
		t.Fatal(err)
	}
	script := `
// Just enough DOM for the import chain: dialog.js registers a document-level
// Escape listener when it loads, and dom.js creates elements.
globalThis.document = {
  addEventListener() {},
  removeEventListener() {},
  getElementById: () => null,
  createElement: () => ({ setAttribute() {}, append() {}, classList: { add() {}, remove() {} } }),
};
globalThis.localStorage = { getItem: () => null, setItem() {} };
const m = await import(process.argv[1]);
process.stdout.write(m.languageLabel(process.argv[2]));
`
	out, err := exec.Command(node, "--input-type=module", "-e", script, module, code).Output()
	if err != nil {
		t.Fatalf("running languageLabel(%q): %v", code, err)
	}
	return string(out)
}

// The account menu replaced three header controls; the old ids must be gone
// from the markup and from the scripts, or a listener throws on load and the
// whole view stops working.
func TestTheOldHeaderControlsAreGone(t *testing.T) {
	for _, path := range []string{"index.html", "js/app.js", "js/views/browser.js", "js/views/login.js"} {
		src := readAsset(t, path)
		for _, gone := range []string{"btn-logout", "btn-profile", "btn-theme", "profile-icon"} {
			if strings.Contains(src, gone) {
				t.Errorf("%s still refers to %s, which no longer exists", path, gone)
			}
		}
	}
}

// TestDarkThemeCoversEverySurface is the test that would have caught the real
// problem with a hand-written dark theme: one component left with a white
// background, which reads as a hole in a dark screen. Every literal colour in
// the component stylesheets must be a variable, and the dark block must define
// the surfaces those variables stand for.
func TestDarkThemeCoversEverySurface(t *testing.T) {
	root := frontendRoot()
	css, err := os.ReadDir(path.Join("..", "..", "internal", "app", "frontend", "css"))
	if err != nil {
		t.Fatal(err)
	}
	literal := regexp.MustCompile(`(?i)(background(?:-color)?|color)\s*:\s*#[0-9a-f]{3,8}`)
	for _, entry := range css {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".css") || entry.Name() == "app.css" {
			continue
		}
		data, err := os.ReadFile(path.Join("..", "..", "internal", "app", "frontend", "css", entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if found := regexp.MustCompile(`(?i)background(?:-color)?\s*:\s*#[0-9a-f]{3,8}`).FindAllString(string(data), -1); found != nil {
			t.Errorf("%s has hardcoded backgrounds %v; use the variables from app.css so the dark theme reaches them", entry.Name(), found)
		}
		// White *text* is allowed on a filled control: a green or red button
		// keeps white lettering in both themes, which is why --text-invert is
		// not used there. Any other literal text colour is a light-theme value.
		for _, decl := range literal.FindAllString(string(data), -1) {
			if regexp.MustCompile(`(?i)^color\s*:\s*#fff(?:fff)?$`).MatchString(decl) {
				continue
			}
			t.Errorf("%s has a hardcoded text colour %q; use a variable from app.css", entry.Name(), decl)
		}
	}
	_ = root

	app := readAsset(t, "css/app.css")
	// The variables a component would reach for. A missing one is undefined in
	// the dark theme, which the browser then paints as nothing at all.
	for _, name := range []string{
		"--text", "--text-light", "--text-invert",
		"--primary", "--primary-dark", "--primary-light",
		"--bg", "--surface", "--border", "--border-strong",
		"--danger", "--warning", "--info",
		"--tint", "--tint-strong",
		"--danger-bg", "--danger-text",
		"--warning-bg", "--warning-text", "--warning-border",
	} {
		if !strings.Contains(app, name+":") {
			t.Errorf("app.css never defines %s", name)
		}
	}
	// Both selectors, because either one alone leaves half the feature out:
	// the attribute alone ignores the system, the media query alone makes the
	// toggle do nothing.
	for _, block := range []string{`@media (prefers-color-scheme: dark)`, `:root[data-theme="dark"]`} {
		if !strings.Contains(app, block) {
			t.Errorf("app.css has no dark block for %s", block)
		}
	}
	if !strings.Contains(app, `:root:not([data-theme="light"])`) {
		t.Error("the media-query block does not let a stored light choice win on a dark machine")
	}
}

// readAsset reads a file from the embedded frontend and fails if it is missing.
func readAsset(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(path.Join("..", "..", "internal", "app", "frontend", name))
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return string(data)
}
