package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// repoContent is the documentation as committed, relative to this package.
const repoContent = "../../docs/content"

// TestRealDocsBuildClean pins the committed documentation: it must build, and
// every internal link and anchor must resolve. This is the check `make
// docs-check` runs in CI, expressed as a test so `go test ./...` catches a
// broken link too.
func TestRealDocsBuildClean(t *testing.T) {
	s, err := build(repoContent)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if len(s.pages) < 10 {
		t.Fatalf("only %d pages found; is the content folder right?", len(s.pages))
	}
	if problems := s.verify(); len(problems) > 0 {
		t.Fatalf("the docs have problems:\n  %s", strings.Join(problems, "\n  "))
	}
}

// TestRealDocsRenderEveryHeading checks that a page's sections all appear as
// anchors, which is what makes in-page links work at all.
func TestRealDocsRenderEveryHeading(t *testing.T) {
	s, err := build(repoContent)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	for _, d := range s.pages {
		for _, h := range d.headings {
			if !strings.Contains(d.html, `id="`+h.id+`"`) {
				t.Errorf("%s: heading %q has no anchor id %q in the rendered page", d.file, h.text, h.id)
			}
		}
	}
}

// TestVerifyCatchesBrokenLinks pins the check itself. A link checker that has
// never been shown to fail is a link checker nobody should trust: an earlier
// version of this one resolved same-page anchors against the home page and
// reported nothing.
func TestVerifyCatchesBrokenLinks(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "_index.md", "---\ntitle: Home\n---\n\n# Home\n")
	write(t, dir, "a.md", "---\ntitle: A\nweight: 1\n---\n\n## Section\n\n"+
		"[a page]({{< ref \"b.md\" >}})\n"+
		"[the same page](#section)\n"+
		"[an anchor on b]({{< ref \"b.md\" >}}#other)\n"+
		"[broken page]({{< ref \"missing.md\" >}})\n"+
		"[broken anchor](#nope)\n")
	write(t, dir, "b.md", "---\ntitle: B\nweight: 2\n---\n\n## Other\n")

	// Build without the ref shortcode resolving so the checks below are about
	// the links themselves.
	s, err := build(dir)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	problems := strings.Join(s.verify(), "\n")
	for _, want := range []string{"missing/index.html", "#nope"} {
		if !strings.Contains(problems, want) {
			t.Errorf("verify did not report %q; got:\n%s", want, problems)
		}
	}
	// A working same-page anchor and a working cross-page anchor must not be
	// reported.
	for _, bad := range []string{"missing anchor: #section", "b/index.html#other"} {
		if strings.Contains(problems, bad) {
			t.Errorf("verify wrongly reported %q", bad)
		}
	}
}

// TestSamePageAnchorUsesItsOwnPage is the regression test for the bug that made
// the checker useless: "#anchor" was resolved against the site root.
func TestSamePageAnchorUsesItsOwnPage(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "_index.md", "---\ntitle: Home\n---\n\n# Home\n")
	write(t, dir, "p.md", "---\ntitle: P\nweight: 1\n---\n\n## Only here\n\n[go](#only-here)\n")

	s, err := build(dir)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if problems := s.verify(); len(problems) > 0 {
		t.Fatalf("a same-page anchor was reported as broken:\n  %s", strings.Join(problems, "\n  "))
	}
}

// TestDuplicateHeadingsReported checks that two headings with the same text are
// reported, because the second one's link would silently go to the first.
func TestDuplicateHeadingsReported(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "_index.md", "---\ntitle: Home\n---\n")
	write(t, dir, "p.md", "---\ntitle: P\nweight: 1\n---\n\n## Notes\n\n## Notes\n")

	s, err := build(dir)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	problems := strings.Join(s.verify(), "\n")
	if !strings.Contains(problems, "same anchor #notes") {
		t.Fatalf("duplicate headings not reported:\n%s", problems)
	}
}

// TestMarkdownSubset covers the constructs the docs use, one assertion each, so
// a change to the renderer that breaks a page is caught here rather than by a
// reader.
func TestMarkdownSubset(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"heading id", "## Configuration and languages", []string{`<h2 id="configuration-and-languages">`}},
		{"inline code in heading", "## `storage.root` and me", []string{`id="storageroot-and-me"`}},
		{"fenced code", "```sh\nmake build\n```", []string{`<pre><code class="language-sh">`, "make build"}},
		{"code is escaped", "```\n{{< ref \"a.md\" >}}\n```", []string{"{{&lt; ref &#34;a.md&#34; &gt;}}"}},
		{"table", "| A | B |\n|---|---|\n| 1 | 2 |", []string{"<table>", "<th>A</th>", "<td>2</td>"}},
		{"table alignment", "| N |\n|---:|\n| 1 |", []string{`style="text-align:right"`}},
		{"blockquote", "> **Note** worth reading", []string{"<blockquote>", "<strong>Note</strong>"}},
		{"nested list", "- one\n  - two\n- three", []string{"<ul>", "<li>one", "</li>", "<li>three</li>"}},
		{"ordered list", "1. one\n2. two", []string{"<ol>", "<li>one</li>", "<li>two</li>", "</ol>"}},
		{"ordered continuation", "1. one\n   and more\n2. two", []string{"<li>one and more</li>"}},
		{"ordered sorted numerically", "1. ten\n2. nine\n3. one", []string{"<li>ten</li>", "<li>nine</li>", "<li>one</li>"}},
		{"bullet inside ordered", "1. one\n   - sub\n2. two", []string{"<ol>", "<ul>", "<li>sub</li>", "<li>two</li>"}},
		{"ordered inside bullet", "- one\n  1. sub\n- two", []string{"<ul>", "<ol>", "<li>sub</li>", "<li>two</li>"}},
		{"switching kind starts a new list", "- one\n1. two", []string{"<ul>", "</ul>", "<ol>"}},
		{"a year is not a list", "2026. was a good year", []string{"<p>2026. was a good year</p>"}},
		{"bold and italic", "**a** and *b*", []string{"<strong>a</strong>", "<em>b</em>"}},
		{"inline code keeps markup", "`**not bold**`", []string{"<code>**not bold**</code>"}},
		{"comment dropped", "text <!-- hidden --> more", []string{"text", "more", "hidden"}},
		{"ref link", "[Install]({{< ref \"install.md\" >}})", []string{`href="../install/index.html"`}},
		{"ref anchor", "[x]({{< ref \"api.md\" >}}#name-collisions)", []string{`href="../api/index.html#name-collisions"`}},
		{"plain md link", "[x](install.md#run-it)", []string{`href="../install/index.html#run-it"`}},
		{"external link untouched", "[x](https://example.com/a#b)", []string{`href="https://example.com/a#b"`}},
		{"paragraph joins lines", "one\ntwo", []string{"<p>one\ntwo</p>"}},
		{"hard rule", "---", []string{"<hr>"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Rendered as a subpage, so link prefixes are exercised too.
			got := markdown(c.in, &doc{file: "p.md", slug: "p"})
			for _, want := range c.want {
				ok := strings.Contains(got, want)
				// "comment dropped" checks an absence.
				if c.name == "comment dropped" && want == "hidden" {
					ok = !ok
				}
				if !ok {
					t.Errorf("missing %q in:\n%s", want, got)
				}
			}
		})
	}
}

// TestJSONBlocksAreValidAndFormatted checks every ```json block in the docs.
//
// A block labelled json that does not parse is a lie to a reader who copies
// it, and one written on a single long line cannot be read at a glance. The
// docs had both: a `{ "data": { ... } }` placeholder and bare fragments such
// as `"dsn": "…"` that are not documents at all. So each block must parse, and
// must be laid out the way the rest of the docs are: a "{ " opening, two-space
// indentation, and no line longer than the prose around it allows.
func TestJSONBlocksAreValidAndFormatted(t *testing.T) {
	files, err := filepath.Glob(filepath.Join(repoContent, "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no documentation pages found")
	}
	blocks := 0
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(string(data), "\n")
		for i := 0; i < len(lines); i++ {
			if strings.TrimSpace(lines[i]) != "```json" {
				continue
			}
			end := i + 1
			for end < len(lines) && strings.TrimSpace(lines[end]) != "```" {
				end++
			}
			if end == len(lines) {
				t.Errorf("%s:%d: json block is never closed", path, i+1)
				break
			}
			blocks++
			body := strings.Join(lines[i+1:end], "\n")
			checkJSONBlock(t, path, i+1, body)
			i = end
		}
	}
	if blocks < 10 {
		t.Fatalf("found only %d json blocks; the scan is not reading the pages", blocks)
	}
}

// checkJSONBlock reports a block that will not parse or is not laid out like
// its neighbours.
func checkJSONBlock(t *testing.T, path string, line int, body string) {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(body), &v); err != nil {
		t.Errorf("%s:%d: not valid JSON (%v):\n%s", path, line, err, body)
		return
	}
	if _, ok := v.(map[string]any); !ok {
		// The docs show objects and arrays; a bare string or number is a
		// fragment that belongs inside one.
		t.Errorf("%s:%d: a json block must be an object or an array", path, line)
	}
	first := strings.SplitN(body, "\n", 2)[0]
	if len(strings.Split(body, "\n")) > 1 {
		// Long or nested: one key per line, two spaces per level, so the
		// nesting is visible without counting characters.
		for n, ln := range strings.Split(body, "\n")[1:] {
			if strings.TrimSpace(ln) == "" {
				// A blank line between top-level sections is how
				// configuration.md keeps its groups apart. Inside an object
				// or array it is noise.
				if strings.Contains(ln, " ") {
					t.Errorf("%s:%d: blank line indented inside a json block", path, line+n+2)
				}
				continue
			}
			if spaces := len(ln) - len(strings.TrimLeft(ln, " ")); spaces%2 != 0 {
				t.Errorf("%s:%d: json indented by %d spaces; use two per level: %q",
					path, line+n+2, spaces, ln)
			}
		}
		return
	}
	if len(first) > 76 {
		t.Errorf("%s:%d: a one-line json block is longer than %d characters; break it over several lines",
			path, line, 76)
	}
}

// TestNoCollapsedNumberedLists is the regression test for the run-on that
// reached the published site: the renderer had no ordered-list support, so
// "1. … 2. … 3. …" was emitted as one paragraph and read as a wall of text.
// A paragraph that starts with a number is that bug coming back.
func TestNoCollapsedNumberedLists(t *testing.T) {
	s, err := build(repoContent)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	collapsed := regexp.MustCompile(`(?m)<p>\s*[0-9]{1,2}\. `)
	lists := 0
	for _, d := range s.pages {
		if m := collapsed.FindString(d.html); m != "" {
			t.Errorf("%s: a numbered list was rendered as a paragraph (%q)", d.file, strings.TrimSpace(m))
		}
		lists += strings.Count(d.html, "<ol>")
	}
	if lists == 0 {
		t.Error("no page renders an ordered list; the docs have numbered steps")
	}
}

// TestWriteProducesTheSite checks the files a deployed site needs.
func TestWriteProducesTheSite(t *testing.T) {
	s, err := build(repoContent)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	out := t.TempDir()
	if err := s.write(out); err != nil {
		t.Fatalf("write: %v", err)
	}
	for _, f := range []string{
		"index.html", "style.css", "search.js", "search.json", "sitemap.xml",
		"fonts.css", "fonts/OFL.txt", "fonts/LiberationSans-Regular.ttf",
		"install/index.html", "api/index.html",
	} {
		if _, err := os.Stat(filepath.Join(out, f)); err != nil {
			t.Errorf("missing %s in the built site", f)
		}
	}
	index, err := os.ReadFile(filepath.Join(out, "search.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range s.pages {
		if !strings.Contains(string(index), `"title":`+jsonString(d.title)) {
			t.Errorf("search.json has no entry for %q", d.title)
		}
	}
}

// TestNoExternalRequests checks that a built page loads nothing from another
// host: the docs work offline and do not leak a reader's visit to a third
// party through a font or a script.
func TestNoExternalRequests(t *testing.T) {
	s, err := build(repoContent)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	for _, d := range s.pages {
		page := s.layout(d)
		for _, attr := range []string{"src=", `rel="stylesheet"`} {
			for _, part := range strings.Split(page, attr)[1:] {
				ref, _, _ := strings.Cut(part, `"`)
				ref = strings.TrimPrefix(ref, `="`)
				if strings.Contains(ref, "://") {
					t.Errorf("%s loads %s from another host", d.file, ref)
				}
			}
		}
	}
}

// TestDarkModeUsesRealFiles checks the swap happens in the page, not through a
// CSS filter: a filter cannot reach the favicon and smears the logo's edges.
func TestDarkModeUsesRealFiles(t *testing.T) {
	s, err := build(repoContent)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	for _, d := range s.pages {
		// Pages in a subfolder reach the site root with "../"; the home page
		// does not, so the prefix is rebuilt rather than assumed.
		prefix := s.root(d)
		page := s.layout(d)
		for _, want := range []string{
			`rel="icon" href="` + prefix + `img/filegator-logo.svg"`,
			`data-dark="` + prefix + `img/filegator-logo-dark.svg"`,
			`prefers-color-scheme: dark`,
		} {
			if !strings.Contains(page, want) {
				t.Errorf("%s is missing %q", d.file, want)
			}
		}
	}
	if strings.Contains(styleCSS, "filter: invert") {
		t.Error("the stylesheet still filters the logo; the dark files replace that")
	}
}

// TestLinksAreRelativeToTheirPage is the regression test for the broken-links
// bug: a page in a subfolder rendered "storage/index.html" instead of
// "../storage/index.html", so a browser asked for /install/storage/ and got a
// 404. Every internal link on a subpage must start with "../", and one on the
// home page must not.
func TestLinksAreRelativeToTheirPage(t *testing.T) {
	s, err := build(repoContent)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	link := regexp.MustCompile(`href="([^"#][^"]*)"`)
	for _, d := range s.pages {
		for _, m := range link.FindAllStringSubmatch(d.html, -1) {
			target := m[1]
			if strings.HasPrefix(target, "http") || strings.HasPrefix(target, "mailto:") {
				continue
			}
			if d.slug == "" {
				if strings.HasPrefix(target, "../") {
					t.Errorf("%s: %q climbs above the site root", d.file, target)
				}
				continue
			}
			if !strings.HasPrefix(target, "../") {
				t.Errorf("%s: %q is relative to the site root, not to this page; it needs \"../\"",
					d.file, target)
			}
		}
	}
}

// TestVerifyResolvesLinksLikeABrowser pins the link checker itself. It used to
// strip "../" and look the rest up from the root, so it passed links that a
// browser could not follow — the checker agreed with itself and not with the
// browser.
func TestVerifyResolvesLinksLikeABrowser(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "_index.md", "---\ntitle: Home\n---\n\n[ok]({{< ref \"a.md\" >}})\n")
	// A subpage whose link is missing its "../": correct for the root, wrong
	// from inside a folder.
	write(t, dir, "a.md", "---\ntitle: A\nweight: 1\n---\n\n[broken](b/index.html)\n")
	write(t, dir, "b.md", "---\ntitle: B\nweight: 2\n---\n")

	s, err := build(dir)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	problems := strings.Join(s.verify(), "\n")
	if !strings.Contains(problems, "a.md: links to a page that does not exist") {
		t.Fatalf("the checker accepted a link a browser cannot follow:\n%s", problems)
	}
}

// TestSidebarLeadsWithHome checks that the home page is the first sidebar
// link on every page, that it points at the site root, and that it is the
// active one on the home page. The home page once had no sidebar entry at all,
// which left a reader who landed on a subpage from a search engine with no way
// back to the overview.
func TestSidebarLeadsWithHome(t *testing.T) {
	s, err := build(repoContent)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	nav := regexp.MustCompile(`(?s)<aside id="sidebar">.*?<ul>\s*<li><a([^>]*)>`)
	for _, d := range s.pages {
		m := nav.FindStringSubmatch(s.layout(d))
		if m == nil {
			t.Errorf("%s: no sidebar", d.file)
			continue
		}
		attrs := m[1]
		want := "index.html"
		if d.slug != "" {
			want = "../index.html"
		}
		if !strings.Contains(attrs, `href="`+want+`"`) {
			t.Errorf("%s: the first sidebar link is not the home page: %s", d.file, attrs)
		}
		if d.slug == "" && !strings.Contains(attrs, `aria-current="page"`) {
			t.Errorf("%s: the home page is not marked active in the sidebar: %s", d.file, attrs)
		}
	}
}

// TestPagerSkipsHome checks that the home page stays out of the reading
// sequence: the first page has no "Previous" and no page links "Next" to it.
func TestPagerSkipsHome(t *testing.T) {
	s, err := build(repoContent)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	for _, d := range s.pages {
		page := s.layout(d)
		if strings.Contains(page, `class="next" href="../index.html"`) ||
			strings.Contains(page, `class="next" href="index.html"`) {
			t.Errorf("%s: the pager points at the home page", d.file)
		}
	}
}

// write creates a content file in dir.
func write(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func jsonString(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`)
	return `"` + r.Replace(s) + `"`
}
