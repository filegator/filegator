package app

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"path"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// importPattern matches ES module import specifiers.
var importPattern = regexp.MustCompile(`(?m)^\s*(?:import|export)\s[^;]*?from\s+"([^"]+)"`)

// TestModuleImportsResolve guards against broken relative imports, which show
// up in the browser as a confusing "disallowed MIME type" error because the
// missing file returns a 404.
func TestModuleImportsResolve(t *testing.T) {
	root := frontendRoot()

	err := fs.WalkDir(root, "js", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".js") {
			return err
		}
		data, readErr := fs.ReadFile(root, p)
		if readErr != nil {
			return readErr
		}
		for _, match := range importPattern.FindAllStringSubmatch(string(data), -1) {
			spec := match[1]
			if !strings.HasPrefix(spec, ".") {
				// Bare specifiers are not used: no bundler resolves them.
				t.Errorf("%s imports %q; only relative imports are supported", p, spec)
				continue
			}
			target := path.Join(path.Dir(p), spec)
			if _, statErr := fs.Stat(root, target); statErr != nil {
				t.Errorf("%s imports %q which does not exist (looked for %s)", p, spec, target)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
}

// TestIndexScriptIsModule checks the entrypoint is loaded as a module.
func TestIndexScriptIsModule(t *testing.T) {
	data, err := fs.ReadFile(frontendRoot(), "index.html")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `type="module"`) {
		t.Error("index.html must load its scripts with type=\"module\"")
	}
}

// TestMissingAssetKeepsContentType ensures a missing .js file answers with a
// JavaScript content type, so the browser reports a 404 rather than a MIME
// type error.
func TestMissingAssetKeepsContentType(t *testing.T) {
	cases := map[string]string{
		"/js/missing.js":          "text/javascript",
		"/js/views/missing.js":    "text/javascript",
		"/css/missing.css":        "text/css",
		"/js/nested/deep/miss.js": "text/javascript",
	}
	for p, want := range cases {
		rec := get(newTestApp(t), p)
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: status %d, want 404", p, rec.Code)
			continue
		}
		if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, want) {
			t.Errorf("%s: content type %q, want %q", p, got, want)
		}
	}
}

// TestHeadAsset works because browsers and proxies may use HEAD.
func TestHeadAsset(t *testing.T) {
	app := newTestApp(t)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, newHeadRequest("/js/app.js"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
}

// TestRealFilesParseLoad protects against a file that exists but is empty or
// truncated.
func TestRealFilesParseLoad(t *testing.T) {
	var files []string
	err := fs.WalkDir(frontendRoot(), "js", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".js") {
			return err
		}
		files = append(files, p)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) < 8 {
		t.Fatalf("only found %d modules: %v", len(files), files)
	}
	for _, p := range files {
		info, err := fs.Stat(frontendRoot(), p)
		if err != nil {
			t.Fatal(err)
		}
		if info.Size() == 0 {
			t.Errorf("%s is empty", p)
		}
	}
}

func newHeadRequest(target string) *http.Request {
	r, _ := http.NewRequest(http.MethodHead, target, nil)
	return r
}

// TestNoHardcodedAPIPaths guards the subpath rule: the frontend may be served
// under a prefix, and only modules/api.js knows where that is. Any other file
// writing "/api/..." into a request bypasses url() and breaks under a prefix —
// invisibly, because the page still loads.
func TestNoHardcodedAPIPaths(t *testing.T) {
	root := frontendRoot()

	// Matches a request target like fetch("/api/..."), request(`/api/...`),
	// or a template literal. Comments and prose are not requests, so the
	// check looks for the call forms.
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`fetch\(\s*["'` + "`" + `]/api/`),
		regexp.MustCompile(`request\(\s*["'` + "`" + `]/api/`),
		regexp.MustCompile(`location\.(?:href|assign)\s*=\s*["'` + "`" + `]/api/`),
	}

	err := fs.WalkDir(root, "js", func(name string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() || !strings.HasSuffix(name, ".js") {
			return walkErr
		}
		if name == "js/modules/api.js" {
			// The API client is the one place allowed to name API paths.
			return nil
		}
		data, err := fs.ReadFile(root, name)
		if err != nil {
			return err
		}
		for i, line := range strings.Split(string(data), "\n") {
			for _, re := range patterns {
				if re.MatchString(line) {
					t.Errorf("%s:%d calls the API without api.url(): %s",
						name, i+1, strings.TrimSpace(line))
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestModeColumnIsGone pins the decision that the table has no mode column:
// the octal mode lives in the row menu as "Mode (644)", next to the
// action that changes it, which is what v7 did too.
func TestModeColumnIsGone(t *testing.T) {
	page, err := fs.ReadFile(frontendRoot(), "index.html")
	if err != nil {
		t.Fatal(err)
	}
	for _, gone := range []string{"col-mode", "sort-mode", ">Mode<"} {
		if strings.Contains(string(page), gone) {
			t.Errorf("index.html still contains %q; the mode column was removed", gone)
		}
	}

	browserJS, err := fs.ReadFile(frontendRoot(), "js/views/browser.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, gone := range []string{"col-mode", "modeCell", "showSymbolic", "mode-button", `"mode"`} {
		if strings.Contains(string(browserJS), gone) {
			t.Errorf("browser.js still references %q", gone)
		}
	}

	// The dialog keeps the symbolic preview, so the helper must remain.
	mod, err := fs.ReadFile(frontendRoot(), "js/modules/permissions.js")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(mod), "export function symbolic") {
		t.Error("symbolic() is still used by the permissions dialog and must not be removed")
	}
}

// TestModeMenuActionShowsMode checks the label carries the octal mode, and
// degrades to the bare word when an entry has none.
func TestModeMenuActionShowsMode(t *testing.T) {
	data, err := fs.ReadFile(frontendRoot(), "js/modules/row_actions.js")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	if !strings.Contains(source, "Mode (${mode})") {
		t.Error(`row_actions.js should label the chmod action "Mode (644)"`)
	}
	if !strings.Contains(source, "mode ? `Mode (${mode})` : \"Mode\"") {
		t.Error("an entry without a mode should get the bare \"Mode\" label")
	}
	if strings.Contains(source, "Permissions (") {
		t.Error("the menu action was renamed from \"Permissions\" to \"Mode\"")
	}
}

// adminTabOrder captures the tab list literal in admin.js, and adminTabID the
// id of each entry in it.
var (
	adminTabOrder = regexp.MustCompile(`(?s)const items = \[(.*?)\];`)
	adminTabID    = regexp.MustCompile(`\["(\w+)",`)
)

// TestAdminNavIsBelowTheHeader pins the layout decision of 2026-10-26: the
// administration sections are a strip below the header, where the file browser
// puts its pathbar, not the middle of the header line. Putting them back in the
// header squeezes five destinations between the logo and the version.
func TestAdminNavIsBelowTheHeader(t *testing.T) {
	page, err := fs.ReadFile(frontendRoot(), "index.html")
	if err != nil {
		t.Fatal(err)
	}
	markup := string(page)
	headerEnd := strings.Index(markup, `id="admin-view"`)
	navAt := strings.Index(markup, `id="admin-tabs"`)
	headerClose := strings.Index(markup[headerEnd:], "</header>")
	if headerEnd < 0 || navAt < 0 || headerClose < 0 {
		t.Fatal("the admin view is missing its header or its section navigation")
	}
	if navAt < headerEnd+headerClose {
		t.Error("the section navigation must be a strip below the header, not inside it")
	}
	if !strings.Contains(markup, `id="admin-tabs" class="admin-nav"`) {
		t.Error(`the section navigation should carry class="admin-nav"`)
	}

	adminJS, err := fs.ReadFile(frontendRoot(), "js/views/admin.js")
	if err != nil {
		t.Fatal(err)
	}
	source := string(adminJS)
	// The sections are labelled <- Back, Users, UI, Files, Security,
	// Repository, Auth. The administration area is English only (user
	// decision 2026-10-27), so its labels are plain strings and the arrow is
	// markup beside them rather than part of a translated key.
	if !strings.Contains(source, `button("Back", () => onClose && onClose(), "tab tab-back")`) {
		t.Error(`the way out of the admin area should read "← Back"`)
	}
	if strings.Contains(source, `t("← Back")`) || strings.Contains(source, `t("← Files")`) {
		t.Error("the arrow belongs in the markup, not in a translation key")
	}
	// The order is asserted so a page cannot move without the change being
	// visible here: UI and Files came from the old single Settings page, and
	// Security was split out of it.
	items := adminTabOrder.FindStringSubmatch(source)
	if items == nil {
		t.Fatal("the admin tab list could not be found in admin.js")
	}
	want := []string{"users", "ui", "files", "security", "storage", "auth"}
	var got []string
	for _, match := range adminTabID.FindAllStringSubmatch(items[1], -1) {
		got = append(got, match[1])
	}
	if !strings.Contains(source, `["auth", "Auth"]`) {
		t.Error(`the authentication section should read "Auth"`)
	}
	if strings.Contains(source, `"Settings"`) {
		t.Error(`the Settings section was renamed to "UI"`)
	}
	// Storage is the section name only in the wording; the tab id stays
	// "storage" because it names the config block and renderStorage().
	if !strings.Contains(source, `["storage", "Repository"]`) {
		t.Error(`the Storage section was renamed to "Repository"`)
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("admin tabs = %v, want %v", got, want)
	}
}

// TestAdminIsEnglishOnly pins the decision of 2026-10-27: the administration
// area does not translate. Its labels are English literals, and the keys it
// used to need are gone from every language file, so a translator never sees
// a string they cannot change anything with.
func TestAdminIsEnglishOnly(t *testing.T) {
	data, err := fs.ReadFile(frontendRoot(), "js/views/admin.js")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)

	// The tabs and their labels are the clearest example: plain English.
	for _, label := range []string{`["users", "Users"]`, `["ui", "UI"]`, `["files", "Files"]`,
		`["security", "Security"]`, `["storage", "Repository"]`, `["auth", "Auth"]`} {
		if !strings.Contains(source, label) {
			t.Errorf("admin.js should label the section with %s", label)
		}
	}

	// No translated string at all. The Users table kept calling t("Username")
	// after the first pass, so the whole file is checked rather than a list of
	// known labels.
	if calls := translatedKeysPattern.FindAllString(source, -1); len(calls) > 0 {
		t.Errorf("admin.js still translates %d strings, e.g. %s", len(calls), calls[0])
	}

	// Admin-only keys must not come back to english.json. A key the browser
	// also uses stays (it is translated there), which is why this lists the
	// administration vocabulary that has no place outside the admin area.
	english, err := fs.ReadFile(frontendRoot(), "translations/english.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, gone := range []string{
		"IP allowlist", "IP denylist", "CSRF protection", "Secure cookies (HTTPS only)",
		"Response headers", "Trusted IP header", "Logo URL", "Interface folder",
		"Session lifetime (hours)", "Lockout time (seconds)", "Storage type",
		"Access key", "Secret key", "Bind DN", "Base DN", "Users file",
	} {
		if strings.Contains(string(english), `"`+gone+`"`) {
			t.Errorf("english.json still has the admin-only key %q", gone)
		}
	}
}

// adminFunction returns the source of one render function in admin.js, from its
// declaration to the next top-level function.
func adminFunction(t *testing.T, source, name string) string {
	t.Helper()
	marker := "async function " + name + "("
	start := strings.Index(source, marker)
	if start < 0 {
		t.Fatalf("admin.js has no %s()", name)
	}
	rest := source[start+len(marker):]
	if next := strings.Index(rest, "\n  function "); next >= 0 {
		rest = rest[:next]
	}
	if next := strings.Index(rest, "\n  async function "); next >= 0 {
		rest = rest[:next]
	}
	return source[start : start+len(marker)+len(rest)]
}

// adminPayload returns the object literal a render function sends to the
// settings endpoint, so a test can check exactly which fields a page writes.
func adminPayload(t *testing.T, source, name string) string {
	t.Helper()
	body := adminFunction(t, source, name)
	at := strings.Index(body, "const payload = {")
	if at < 0 {
		t.Fatalf("%s() does not build a payload", name)
	}
	payload := body[at:]
	end := strings.Index(payload, "};")
	if end < 0 {
		t.Fatalf("%s() has no end to its payload", name)
	}
	return payload[:end]
}

// TestAdminPagesSaveOnlyTheirOwnFields guards the reason the server merges a
// partial settings request: each page sends the fields it shows and nothing
// else. A field a page does not show must not be in its payload, or saving it
// would overwrite another page's values with the copies read when the page
// was opened.
func TestAdminPagesSaveOnlyTheirOwnFields(t *testing.T) {
	data, err := fs.ReadFile(frontendRoot(), "js/views/admin.js")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)

	cases := []struct {
		fn   string
		want []string
	}{
		{
			fn: "renderUI",
			want: []string{
				"app_name", "language", "language_selection", "logo",
				"logo_dark", "date_format", "frontend_dir", "guest_redirection",
				"max_width", "allow_password_change", "allow_logout",
			},
		},
		{
			fn: "renderFiles",
			want: []string{
				"upload_max_size", "upload_chunk_size", "upload_simultaneous",
				"default_archive_name", "editable", "download_inline",
			},
		},
		{
			fn: "renderSecurity",
			want: []string{
				"ip_allowlist", "ip_denylist", "trusted_ip_header", "session_ttl",
				"secure_cookies", "csrf_protection", "lockout_attempts",
				"lockout_timeout", "logging_enabled", "logging_file",
				"logging_level", "logging_max_size", "headers",
			},
		},
	}
	for _, c := range cases {
		payload := adminPayload(t, source, c.fn)
		if len(payload) < 100 {
			t.Fatalf("%s payload is suspiciously short (%d chars): %q", c.fn, len(payload), payload)
		}
		for _, field := range c.want {
			if !strings.Contains(payload, field) {
				t.Errorf("%s payload is missing %s", c.fn, field)
			}
		}
		// And nothing from another page.
		for _, other := range cases {
			if other.fn == c.fn {
				continue
			}
			for _, field := range other.want {
				if slices.Contains(c.want, field) {
					continue
				}
				if strings.Contains(payload, field+":") {
					t.Errorf("%s payload carries %s, which belongs to %s", c.fn, field, other.fn)
				}
			}
		}
	}

	// The inline download list is on the Files page, with a live warning when
	// a listed extension can carry script (v7 honoured any extension; the
	// warning is what makes that choice visible).
	filesPage := adminFunction(t, source, "renderFiles")
	if !strings.Contains(filesPage, `"Inline download extensions"`) {
		t.Error("the Files page should offer Inline download extensions")
	}
	if !strings.Contains(filesPage, "is-warning") || !strings.Contains(filesPage, "RISKY") {
		t.Error("the Files page should warn when a listed inline type can contain script")
	}
	if !strings.Contains(filesPage, "download_inline") {
		t.Error("the Files page should be able to save download_inline")
	}

	// The security fields left the UI page with the split, and the interface
	// width stayed on it by decision.
	ui := adminFunction(t, source, "renderUI")
	for _, gone := range []string{
		"ip_allowlist", "session_ttl", "logging_level", "headers.values()",
		"lockout_attempts", "csrf_protection",
	} {
		if strings.Contains(ui, gone) {
			t.Errorf("renderUI() still handles %s; it belongs to the Security page", gone)
		}
	}
	if !strings.Contains(ui, "max_width") {
		t.Error("the interface width stayed on the UI page by decision")
	}
	// The two account switches live on UI by decision (2026-10-26): they
	// change what the signed-in user's header offers, not who may connect.
	for _, want := range []string{"allow_password_change", "allow_logout"} {
		if !strings.Contains(ui, want) {
			t.Errorf("the UI page should handle %s", want)
		}
		if strings.Contains(adminFunction(t, source, "renderSecurity"), want) {
			t.Errorf("renderSecurity() still handles %s; it belongs to the UI page", want)
		}
	}

	// Storage no longer shows a placeholder while it loads.
	storage := adminFunction(t, source, "renderStorage")
	if strings.Contains(storage, `t("Loading")`) {
		t.Error(`renderStorage() still shows a "Loading" line`)
	}
}

// TestUserRowActionOrder pins the order an administrator asked for on
// 2026-10-26: Set password, Edit, Delete, with Delete last and apart. The guest
// row keeps its own Enable/Disable + Edit pair, since it cannot be deleted.
func TestUserRowActionOrder(t *testing.T) {
	data, err := fs.ReadFile(frontendRoot(), "js/views/admin.js")
	if err != nil {
		t.Fatal(err)
	}
	body := adminFunction(t, string(data), "renderUsers")

	// The guest row has an Edit of its own further up, so the user row is
	// compared from its Set password button onwards.
	user := strings.Index(body, `button("Set password"`)
	tail := body[user:]
	edit := strings.Index(tail, `button("Edit"`)
	del := strings.Index(tail, `button("Delete"`)
	if user < 0 || edit < 0 || del < 0 {
		t.Fatalf("a user row is missing one of its actions (set password %d, edit %d, delete %d)", user, edit, del)
	}
	if !(0 <= edit && edit < del) {
		t.Error("user row actions should be ordered Set password, Edit, Delete")
	}
	if !strings.Contains(body, `button("Delete", () => deleteUser(user), "ghost danger")`) {
		t.Error("Delete should stay the danger action")
	}
}

// TestDownloadsOpenInANewTab pins the decision of 2026-10-26 (v7 parity): a
// download never navigates the file manager away. In place, an inline PDF takes
// over the tab and Back can re-trigger it, and an attachment leaves a blank page
// behind. Every download must go through api.openDownload; a stray
// window.location.href is the regression to catch.
func TestDownloadsOpenInANewTab(t *testing.T) {
	for _, file := range []string{"js/views/browser.js", "js/views/gallery.js"} {
		data, err := fs.ReadFile(frontendRoot(), file)
		if err != nil {
			t.Fatal(err)
		}
		source := string(data)
		if strings.Contains(source, "window.location.href = api.") {
			t.Errorf("%s navigates to a download in place; use api.openDownload", file)
		}
	}

	browserJS, err := fs.ReadFile(frontendRoot(), "js/views/browser.js")
	if err != nil {
		t.Fatal(err)
	}
	// The three download entry points: the name link, the row menu and the
	// batch download.
	if got := strings.Count(string(browserJS), "api.openDownload("); got != 3 {
		t.Errorf("browser.js uses openDownload %d times, want 3 (name, row menu, batch)", got)
	}

	// The gallery's Download is a link with a real href that opens in a new
	// tab, and it must not carry the download attribute: that would save the
	// file in place and break the "opens in a tab" promise.
	galleryJS, err := fs.ReadFile(frontendRoot(), "js/views/gallery.js")
	if err != nil {
		t.Fatal(err)
	}
	gallery := string(galleryJS)
	if !strings.Contains(gallery, `attrs: { target: "_blank", rel: "noopener" }`) {
		t.Error("the gallery Download link should open in a new tab")
	}
	if strings.Contains(gallery, "setAttribute(\"download\"") || strings.Contains(gallery, `attrs: { download: "" }`) {
		t.Error("the gallery Download must not download in place")
	}

	// And the helper opens a tab rather than throwing away its result.
	apiJS, err := fs.ReadFile(frontendRoot(), "js/modules/api.js")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(apiJS), "openDownload: (target) => {") {
		t.Error("api.js should provide openDownload")
	}
}

// TestUnusedTranslationKeysAreGone keeps the language files free of the keys
// that were only ever used by the removed toggle, and checks the "Mode" labels
// have English entries to fall back to.
func TestUnusedTranslationKeysAreGone(t *testing.T) {
	data, err := fs.ReadFile(frontendRoot(), "translations/english.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, stale := range []string{"Show symbolic format", "Hide symbolic format", "Change permissions for {0}"} {
		if strings.Contains(string(data), stale) {
			t.Errorf("english.json still carries %q", stale)
		}
	}
	// Without these every language, English included, would print the raw key.
	for _, want := range []string{`"Mode": "Mode"`, `"Change mode for {0}": "Change mode for {0}"`} {
		if !strings.Contains(string(data), want) {
			t.Errorf("english.json is missing %s", want)
		}
	}
}

// TestAPIClientUsesURL guards the other half: every request in api.js must
// reach url(), so the prefix is applied once rather than per call site.
func TestAPIClientUsesURL(t *testing.T) {
	data, err := fs.ReadFile(frontendRoot(), "js/modules/api.js")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)

	// Every request(...) whose first argument is an API path must wrap it.
	calls := regexp.MustCompile(`request\(\s*(` + "`" + `|"|')/api/`)
	if found := calls.FindAllString(source, -1); len(found) > 0 {
		t.Errorf("%d request() calls pass an unprefixed API path", len(found))
	}
}

// TestTreeIsResetBetweenAccounts guards a folder tree that outlived the
// session: the browser view is created once per page, so after a logout the
// previous user's folders were still listed for the next account, and opening
// one failed with "path not found".
//
// Three links keep it from happening again: the tree view offers reset(), the
// browser rebuilds it when the user changes, and app.js asks for both on
// logout — including the URL, which would otherwise be restored on the next
// show() and 404.
func TestTreeIsResetBetweenAccounts(t *testing.T) {
	read := func(name string) string {
		t.Helper()
		data, err := fs.ReadFile(frontendRoot(), name)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}

	tree := read("js/modules/tree.js")
	for _, want := range []string{"async function reset(", "\n    reset,", "root.children = null;"} {
		if !strings.Contains(tree, want) {
			t.Errorf("tree.js should offer a reset that drops the loaded folders (missing %q)", want)
		}
	}

	browser := read("js/views/browser.js")
	if !strings.Contains(browser, "if (!user || nextUser.username !== user.username)") {
		t.Error("browser.js should start clean when a different account signs in")
	}
	if !strings.Contains(browser, "treeView.element.remove();") {
		t.Error("browser.js should discard the previous account's tree element")
	}
	if !strings.Contains(browser, `history.replaceState(null, "", window.location.pathname);`) {
		t.Error("browser.js reset() should clear the path from the URL")
	}

	body, err := fs.ReadFile(frontendRoot(), "js/app.js")
	if err != nil {
		t.Fatal(err)
	}
	start := strings.Index(string(body), "function showLogin() {")
	if start < 0 {
		t.Fatal("app.js has no showLogin()")
	}
	rest := string(body)[start:]
	if next := strings.Index(rest, "\nasync function "); next >= 0 {
		rest = rest[:next]
	}
	if !strings.Contains(rest, "browser.reset();") {
		t.Error("showLogin() should reset the browser view before hiding it")
	}
}
