package app

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The logo is drawn by the browser, so most of it is markup and CSS: these
// tests pin the wiring that would fail silently — a missing dark variant, a
// download of the file the theme hid, or a themed logo that stops following the
// theme when the user toggles it.

// TestEveryLogoHasBothThemes: each surface is a group of two images, and the
// group's ids are what js/modules/logo.js fills in. One image per surface was
// the old shape; a dark-mode logo needs a second, and the module addresses both
// by id, so a missing or renamed one leaves that theme without a logo.
func TestEveryLogoHasBothThemes(t *testing.T) {
	page := readAsset(t, "index.html")
	groups := map[string]string{
		"login-logo": "login card",
		"app-logo":   "file browser header",
		"admin-logo": "administration header",
	}
	for group, where := range groups {
		for _, theme := range []string{"light", "dark"} {
			id := `<img id="` + group + `-` + theme + `"`
			at := strings.Index(page, id)
			if at < 0 {
				t.Errorf("the %s has no #%s-%s image", where, group, theme)
				continue
			}
			tag := page[at : at+strings.Index(page[at:], ">")+1]
			if !strings.Contains(tag, "logo-"+theme) {
				t.Errorf("#%s-%s is missing the .logo-%s class, so no rule can show it", group, theme, theme)
			}
			if !strings.Contains(tag, "hidden") {
				t.Errorf("#%s-%s is not hidden in the markup; it would show before JavaScript runs", group, theme)
			}
		}
	}
}

// The hook the dark palette uses. Without both selectors one of the two — the
// system preference, or the user's toggle — stops switching the logo.
func TestTheDarkThemeSwapsTheLogo(t *testing.T) {
	css := readAsset(t, "css/app.css")
	media := css[strings.Index(css, "@media (prefers-color-scheme: dark)"):]
	media = media[:strings.Index(media, ":root[data-theme=")]
	attribute := css[strings.Index(css, `:root[data-theme="dark"]`):]
	attribute = attribute[:strings.Index(attribute, "/* Logos")]

	for name, block := range map[string]string{"the system preference": media, "the theme toggle": attribute} {
		if !strings.Contains(block, "--logo-light: none") || !strings.Contains(block, "--logo-dark: block") {
			t.Errorf("%s does not swap the logo variables", name)
		}
	}
	if !strings.Contains(css, "display: var(--logo-light)") || !strings.Contains(css, "display: var(--logo-dark)") {
		t.Error("app.css does not paint the logo variants from the variables")
	}
	// display:none matters: an image hidden any other way is still downloaded,
	// so every page would request the logo it never shows.
	if strings.Contains(css, "visibility: var(--logo") {
		t.Error("the logo variants must be display:none, not merely invisible: a hidden image is still fetched")
	}
}

// The hidden state wins by !important, so the theme rule has to as well. A
// lesser rule loses whatever its specificity, and the logo would stay visible
// after setVisible(img, false).
func TestTheLogoThemeRuleBeatsTheHiddenUtility(t *testing.T) {
	css := readAsset(t, "css/app.css")
	hidden := strings.Index(css, ".hidden {")
	if hidden < 0 || !strings.Contains(css[hidden:hidden+strings.Index(css[hidden:], "}")], "!important") {
		t.Fatal(".hidden no longer uses !important; revisit the logo rules")
	}
	for _, selector := range []string{".logo-light:not(.hidden)", ".logo-dark:not(.hidden)"} {
		at := strings.Index(css, selector+" {")
		if at < 0 {
			t.Errorf("app.css has no %s rule", selector)
			continue
		}
		rule := css[at : at+strings.Index(css[at:], "}")]
		if !strings.Contains(rule, "!important") {
			t.Errorf("%s does not use !important, so .hidden would override it", selector)
		}
	}
}

// TestThePreloadPicksTheThemeFile: the preload starts the download before the
// first paint, so it must ask for the same file the stylesheet will show.
func TestThePreloadPicksTheThemeFile(t *testing.T) {
	page := readAsset(t, "index.html")
	head := page[:strings.Index(page, "</head>")]
	at := strings.Index(head, `rel = "preload"`)
	if at < 0 {
		at = strings.Index(head, `rel="preload"`)
	}
	if at < 0 {
		t.Fatal("index.html does not preload the logo")
	}
	script := head[:at]
	if !strings.Contains(script, `localStorage.getItem("filegator.theme")`) &&
		!strings.Contains(head, "savedTheme") {
		t.Error("the preload does not consult the stored theme")
	}
	if !strings.Contains(head, `matchMedia("(prefers-color-scheme: dark)")`) {
		t.Error("the preload does not follow the system preference")
	}
	if !strings.Contains(page, "{{BASE_PATH}}") {
		t.Error("the preload does not build the URL from the mount prefix")
	}
}

// TestLogoModuleDrivesEverySurface: the module is the one place that knows the
// group ids, and the views must not go back to setting src themselves.
func TestLogoModuleDrivesEverySurface(t *testing.T) {
	module := readAsset(t, "js/modules/logo.js")
	for _, want := range []string{
		`document.getElementById(group + "-light")`,
		`document.getElementById(group + "-dark")`,
		`"src"`,
	} {
		if !strings.Contains(module, want) {
			t.Errorf("logo.js is missing %q", want)
		}
	}
	// The embedded artwork, for the first paint and for a config that never
	// arrives. It has to be the same two files the server defaults to.
	if !strings.Contains(module, `"/img/filegator-logo.svg"`) ||
		!strings.Contains(module, `"/img/filegator-logo-dark.svg"`) {
		t.Error("logo.js does not carry the embedded logo paths")
	}

	for _, file := range []string{"js/views/browser.js", "js/views/login.js"} {
		src := readAsset(t, file)
		if !strings.Contains(src, "applyLogo(") {
			t.Errorf("%s does not use applyLogo", file)
		}
		if regexp.MustCompile(`\.src\s*=\s*logo`).MatchString(src) {
			t.Errorf("%s sets a logo src itself; the pair is logo.js's job", file)
		}
	}
	if strings.Contains(readAsset(t, "js/views/browser.js"), `el("app-logo")`) {
		t.Error("browser.js still looks up the old single-image #app-logo id")
	}
}

// The light theme's values must be declared on :root, not only inside the dark
// blocks. A variable that is never defined makes `display: var(--logo-light)`
// invalid, and an invalid declaration is dropped — so the element falls back to
// its default display and BOTH logos show at once. That was the reported bug:
// the header rendered the light and the dark wordmark side by side.
func TestTheLightThemeShowsOneLogo(t *testing.T) {
	css := readAsset(t, "css/app.css")
	root := css[:strings.Index(css, "*,")] // the :root block, before the reset
	if !strings.Contains(root, "--logo-light: block") || !strings.Contains(root, "--logo-dark: none") {
		t.Fatal("the light palette on :root does not declare the logo pair; an undefined variable makes both logos visible")
	}
}

// The preload picks the theme's variant, so the two files named there must be
// the two the module falls back to — a page that prefetches one file while the
// markup asks for another downloads both.
func TestThePreloadNamesTheEmbeddedLogos(t *testing.T) {
	page := readAsset(t, "index.html")
	module := readAsset(t, "js/modules/logo.js")
	for _, name := range []string{"filegator-logo.svg", "filegator-logo-dark.svg"} {
		if !strings.Contains(page, name) {
			t.Errorf("index.html does not preload %s", name)
		}
		if !strings.Contains(module, `"/img/`+name+`"`) {
			t.Errorf("logo.js does not use the embedded %s", name)
		}
	}
}

// The embedded files are the docs site's artwork, unchanged: the docs and the
// application must not drift into two drawings.
func TestTheEmbeddedLogosAreTheDocumentedOnes(t *testing.T) {
	for _, name := range []string{"filegator-logo.svg", "filegator-logo-dark.svg"} {
		app, err := fs.ReadFile(frontendRoot(), "img/"+name)
		if err != nil {
			t.Fatalf("the embedded %s is missing: %v", name, err)
		}
		docs, err := os.ReadFile(filepath.Join("..", "..", "docs", "static", "img", name))
		if err != nil {
			t.Fatalf("docs/static/img/%s is missing: %v", name, err)
		}
		if string(app) != string(docs) {
			t.Errorf("internal/app/frontend/img/%s differs from docs/static/img/%s", name, name)
		}
	}
}

// The path must never be hardcoded outside modules/api.js (subpath rule). The
// logo module is the one exception, and it is deliberate: a preloaded image is
// not an API call, and the two paths are the embedded files.
func TestLogoPathsAreNotServedByHand(t *testing.T) {
	for _, file := range []string{"js/views/admin.js", "js/views/browser.js", "js/views/login.js", "js/app.js"} {
		if strings.Contains(readAsset(t, file), "/img/filegator-logo") {
			t.Errorf("%s hardcodes a logo path; ask logo.js or the config", file)
		}
	}
}

// The favicon is the logo. index.html sets the built-in file for the active
// theme before the first paint, so the tab is never iconless; a configured logo
// replaces it. A raster or non-SVG file must not keep the SVG type, or the
// browser reads it as the wrong kind of image.
func TestTheFaviconFollowsTheLogo(t *testing.T) {
	page := readAsset(t, "index.html")
	head := page[:strings.Index(page, "</head>")]
	icon := strings.Index(head, `icon.rel = "icon"`)
	if icon < 0 {
		t.Fatal("index.html does not set a favicon")
	}
	if !strings.Contains(head[icon:], `icon.type = "image/svg+xml"`) {
		t.Error("the favicon link does not declare the SVG type")
	}
	// Relative, so it resolves through <base> like every other asset: an
	// absolute "/img/..." would 404 under -base-path.
	if !strings.Contains(head[icon:], `icon.href = file.slice(1)`) {
		t.Error("the favicon URL should be relative to <base>, not prefixed by hand")
	}

	module := readAsset(t, "js/modules/logo.js")
	for _, want := range []string{
		`link[rel="icon"]`,
		`image/svg+xml`,
		`export function faviconFromConfig`,
	} {
		if !strings.Contains(module, want) {
			t.Errorf("logo.js is missing %q", want)
		}
	}
	// The built-in file when nothing is configured, the configured one otherwise.
	if !strings.Contains(module, `logosFromConfig(config).light || BUILT_IN_LOGO`) {
		t.Error("faviconFromConfig should fall back to the built-in logo")
	}
	app := readAsset(t, "js/app.js")
	if !strings.Contains(app, "applyFavicon(faviconFromConfig(config))") {
		t.Error("app.js never applies the favicon from the configuration")
	}
}

// TestLogoModuleRunsWhatItClaims runs logo.js against a fake DOM, using the
// node already installed on the machine when there is one.
//
// The Go tests can only check that the source mentions the right things. The
// branches that actually decide what a user sees — a pair, a single picture
// reused, no picture, and not re-fetching an unchanged src — are behaviour, and
// the reported "two logos at once" was a behaviour bug that reading the source
// did not catch. The check is skipped, not failed, without node: the project
// has no Node dependency and CI does not install one.
func TestLogoModuleRunsWhatItClaims(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed; the module's behaviour is unchecked on this machine")
	}
	module, err := filepath.Abs(filepath.Join("frontend", "js", "modules", "logo.js"))
	if err != nil {
		t.Fatal(err)
	}

	script := `
// Just enough DOM for logo.js: getElementById, attributes and classList.
function image(id) {
  const attrs = {};
  const classes = new Set(["hidden"]);
  return {
    id, attrs, classes,
    setAttribute(n, v) { attrs[n] = String(v); },
    getAttribute(n) { return Object.prototype.hasOwnProperty.call(attrs, n) ? attrs[n] : null; },
    classList: {
      toggle(n, on) { on ? classes.add(n) : classes.delete(n); },
      contains: (n) => classes.has(n),
    },
  };
}
const images = {};
for (const g of ["login-logo", "app-logo", "admin-logo"]) {
  for (const t of ["light", "dark"]) images[g + "-" + t] = image(g + "-" + t);
}
globalThis.document = { getElementById: (id) => images[id] || null };
// A head with one favicon link already in it, as index.html leaves it.
const head = [];
const icon = {
  attrs: { rel: "icon", type: "image/svg+xml", href: "img/filegator-logo.svg" },
  removeAttribute(n) { delete this.attrs[n]; },
  setAttribute(n, v) { this.attrs[n] = String(v); },
  getAttribute(n) { return Object.prototype.hasOwnProperty.call(this.attrs, n) ? this.attrs[n] : null; },
};
head.push(icon);
globalThis.document.querySelector = () => icon;
globalThis.document.createElement = () => ({ attrs: {}, removeAttribute(n) { delete this.attrs[n]; }, setAttribute(n, v) { this.attrs[n] = String(v); }, getAttribute(n) { return this.attrs[n] ?? null; } });
globalThis.document.head = { appendChild: (n) => head.push(n) };

const logo = await import(process.argv[1]);
const fail = [];
const check = (ok, msg) => { if (!ok) fail.push(msg); };

check(logo.defaultLogos().light === logo.BUILT_IN_LOGO, "default light logo");
check(logo.defaultLogos().dark === logo.BUILT_IN_LOGO_DARK, "default dark logo");
check(logo.logosFromConfig({}).light === "", "missing key means no logo");

// A pair: each image gets its own file, and both are shown (CSS picks one).
logo.applyLogo("app-logo", "/a.svg", "/b.svg", "Home");
check(images["app-logo-light"].attrs.src === "/a.svg", "light src");
check(images["app-logo-dark"].attrs.src === "/b.svg", "dark src");
check(images["app-logo-light"].attrs.alt === "Home", "alt");
check(!images["app-logo-light"].classes.has("hidden"), "light shown");
check(!images["app-logo-dark"].classes.has("hidden"), "dark shown");

// One picture: the dark image must not be left empty.
logo.applyLogo("admin-logo", "/only.svg", "", "");
check(images["admin-logo-dark"].attrs.src === "/only.svg", "dark falls back to the light file");

// No picture: both hidden, so the group disappears.
logo.applyLogo("login-logo", "", "", "");
check(images["login-logo-light"].classes.has("hidden"), "light hidden");
check(images["login-logo-dark"].classes.has("hidden"), "dark hidden");

// Unchanged src is not rewritten, which would restart the download.
let writes = 0;
const real = images["app-logo-light"].setAttribute;
images["app-logo-light"].setAttribute = (n, v) => { if (n === "src") writes++; real(n, v); };
logo.applyLogo("app-logo", "/a.svg", "/b.svg", "Home");
check(writes === 0, "src rewritten although unchanged");

// The favicon: built-in when nothing is configured, the configured logo when
// there is one, and the SVG type dropped for a file that is not one.
check(logo.faviconFromConfig({}) === logo.BUILT_IN_LOGO, "favicon falls back to the built-in logo");
check(logo.faviconFromConfig(null) === logo.BUILT_IN_LOGO, "favicon survives a missing config");
check(logo.faviconFromConfig({ logo: "/brand/logo.png" }) === "/brand/logo.png", "favicon follows the configured logo");
logo.applyFavicon("logo.svg?v=1");
check(icon.attrs.href === "logo.svg?v=1", "favicon href");
check(icon.attrs.type === "image/svg+xml", "svg keeps the icon type");
logo.applyFavicon("/brand/logo.png");
check(icon.attrs.href === "/brand/logo.png", "favicon href for a raster file");
check(!("type" in icon.attrs), "a raster file must not be declared SVG");
check(head.length === 1, "an existing icon link is reused, not duplicated");

if (fail.length) { console.error(fail.join("\n")); process.exit(1); }
`
	cmd := exec.Command(node, "--input-type=module", "-e", script, module)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("logo.js behaviour: %v\n%s", err, out)
	}
}
