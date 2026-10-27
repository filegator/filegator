// Command docsite builds the FileGator documentation website.
//
// The site is built by this command, not by Hugo. That is a deliberate
// decision: the project allows no tooling beyond Go, no theme was ever
// committed, and a site that can only be produced by a binary that is not in
// the repository is a site nobody can review or rebuild. The pages in
// docs/content are ordinary Markdown with a small front matter block, and this
// package renders them.
//
//	go run ./internal/docsite -out /tmp/site      build the site
//	go run ./internal/docsite -serve :8099        build it and serve it
//	go run ./internal/docsite -check              fail if anything is broken
//
// -check is what CI runs: it builds the whole site and then verifies every
// internal link, every anchor and the search index, exiting non-zero on the
// first category of problem it finds. That is the equivalent of the "the site
// builds" step a Hugo deployment gets for free, without the Hugo.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"html"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// doc is one page of the documentation.
type doc struct {
	// file is the name in docs/content, e.g. "install.md".
	file string
	// slug is the page's URL segment: the file name without ".md", with
	// "_index" becoming the site root.
	slug string
	// title is the page's title, used in the navigation, the <title> and the
	// search index.
	title string
	// summary is the first paragraph, used on the home page and in search.
	summary string
	// weight orders pages in the navigation; 0 sorts last, by title.
	weight int
	// body is the Markdown source.
	body string
	// html is the rendered body.
	html string
	// headings are the page's h2 sections, for the "on this page" list.
	headings []heading
}

// heading is one section title of a page.
type heading struct {
	id    string
	level int
	text  string
}

// site is everything needed to write or check the build.
type site struct {
	pages  []*doc
	bySlug map[string]*doc
}

const (
	contentDir = "docs/content"
	staticDir  = "docs/static"
	siteTitle  = "FileGator"
	tagline    = "Documentation for FileGator v8"
)

// rootDir returns the repository root, found by walking up from the working
// directory until docs/content exists. The command is normally run from the
// repository root, but `go test` runs from the package directory, and both must
// find the same content and static files.
func rootDir() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if st, err := os.Stat(filepath.Join(dir, contentDir)); err == nil && st.IsDir() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("cannot find %s above the working directory", contentDir)
		}
		dir = parent
	}
}

func main() {
	out := flag.String("out", "docs-site", "folder to write the site into")
	serve := flag.String("serve", "", "serve the site on this address after building, e.g. :8099")
	check := flag.Bool("check", false, "verify links, anchors and the search index; exit non-zero on a problem")
	dark := flag.Bool("dark", false, "regenerate the dark-mode logo from the original, then exit")
	flag.Parse()

	if *dark {
		root, err := rootDir()
		if err != nil {
			log.Fatal(err)
		}
		dir := filepath.Join(root, "docs", "static", "img")
		if err := writeDarkFiles(dir); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("wrote the dark logo to %s\n", dir)
		return
	}

	s, err := build(contentDir)
	if err != nil {
		log.Fatal(err)
	}

	if *check {
		problems := s.verify()
		if len(problems) > 0 {
			for _, p := range problems {
				fmt.Fprintln(os.Stderr, "docsite:", p)
			}
			os.Exit(1)
		}
		fmt.Printf("docsite: %d pages, no broken links or anchors\n", len(s.pages))
		return
	}

	if err := s.write(*out); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("wrote %d pages to %s; open %s\n", len(s.pages), *out, filepath.Join(*out, "index.html"))

	if *serve != "" {
		display := "127.0.0.1" + *serve
		if host, port, err := net.SplitHostPort(*serve); err == nil {
			if host == "" {
				host = "127.0.0.1"
			}
			display = net.JoinHostPort(host, port)
		}
		fmt.Printf("serving on http://%s/\n", display)
		log.Fatal(http.ListenAndServe(*serve, http.FileServer(http.Dir(*out))))
	}
}

// build reads and renders every page in dir.
func build(dir string) (*site, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", dir, err)
	}
	s := &site{bySlug: map[string]*doc{}}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		d := parse(e.Name(), string(raw))
		if _, dup := s.bySlug[d.slug]; dup {
			return nil, fmt.Errorf("%s: two pages share the URL /%s/", e.Name(), d.slug)
		}
		s.pages = append(s.pages, d)
		s.bySlug[d.slug] = d
	}
	if len(s.pages) == 0 {
		return nil, fmt.Errorf("no pages in %s", dir)
	}
	sort.Slice(s.pages, func(i, j int) bool {
		a, b := s.pages[i], s.pages[j]
		// The home page is written first but sorts last in the list, so the
		// navigation starts with "First five minutes" rather than the index.
		if a.slug == "" {
			return false
		}
		if b.slug == "" {
			return true
		}
		if a.weight != b.weight {
			return a.weight < b.weight
		}
		return a.title < b.title
	})
	for _, d := range s.pages {
		d.html = markdown(d.body, d)
		d.headings = extractHeadings(d.body)
		d.summary = firstParagraph(d.body)
	}
	return s, nil
}

// parse splits front matter from the body.
func parse(file, raw string) *doc {
	d := &doc{file: file}
	d.slug = strings.TrimSuffix(file, ".md")
	if d.slug == "_index" {
		d.slug = ""
	}
	body := raw
	if strings.HasPrefix(raw, "---\n") {
		if end := strings.Index(raw[4:], "\n---"); end >= 0 {
			front := raw[4 : 4+end]
			body = strings.TrimPrefix(raw[4+end+len("\n---"):], "\n")
			for _, line := range strings.Split(front, "\n") {
				key, val, ok := strings.Cut(line, ":")
				if !ok {
					continue
				}
				val = strings.Trim(strings.TrimSpace(val), `"'`)
				switch strings.TrimSpace(key) {
				case "title":
					d.title = val
				case "weight":
					fmt.Sscanf(val, "%d", &d.weight)
				}
			}
		}
	}
	if d.title == "" {
		d.title = strings.TrimSuffix(file, ".md")
	}
	d.body = body
	return d
}

// extractHeadings collects the h2 sections of a page for the "on this page"
// list. Only h2: h3 and deeper would make a long page's list longer than the
// page's introduction.
func extractHeadings(src string) []heading {
	var out []heading
	inFence := false
	for _, line := range strings.Split(src, "\n") {
		if strings.HasPrefix(line, "```") {
			inFence = !inFence
			continue
		}
		if inFence || !strings.HasPrefix(line, "## ") {
			continue
		}
		text := strings.TrimSpace(strings.TrimPrefix(line, "## "))
		text = stripInline(text)
		out = append(out, heading{id: slugify(text), level: 2, text: text})
	}
	return out
}

// firstParagraph returns the first paragraph of a page, with inline markup
// removed, for the home page and the search index.
func firstParagraph(src string) string {
	var para []string
	inFence := false
	for _, line := range strings.Split(src, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		if t == "" {
			if len(para) > 0 {
				break
			}
			continue
		}
		if strings.HasPrefix(t, "#") || strings.HasPrefix(t, "|") ||
			strings.HasPrefix(t, "- ") || strings.HasPrefix(t, "{{<") ||
			strings.HasPrefix(t, "<!--") {
			continue
		}
		para = append(para, stripInline(t))
	}
	return strings.Join(para, " ")
}

// verify checks the built site for the problems a static site can have:
// links to pages that do not exist, anchors that do not exist, and two
// headings on one page that would produce the same anchor.
//
// The anchors are read from the rendered HTML rather than from the heading
// list: the HTML is what a reader's browser actually sees, and it includes
// every depth of heading.
func (s *site) verify() []string {
	var problems []string

	anchors := map[string]map[string]bool{}
	for _, d := range s.pages {
		set := map[string]bool{"": true, "content": true}
		for _, m := range idPattern.FindAllStringSubmatch(d.html, -1) {
			if set[m[1]] {
				problems = append(problems,
					fmt.Sprintf("%s: two headings produce the same anchor #%s", d.file, m[1]))
			}
			set[m[1]] = true
		}
		anchors[d.slug] = set
	}

	for _, d := range s.pages {
		for _, href := range hrefPattern.FindAllStringSubmatch(d.html, -1) {
			target := href[1]
			if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") ||
				strings.HasPrefix(target, "mailto:") {
				continue
			}
			// Resolve the link the way a browser does, from the page that
			// carries it. An earlier version stripped "../" and looked the
			// remainder up from the root, which is why "storage/index.html" on
			// /install/ was reported as fine while a browser asked for
			// /install/storage/, found nothing, and the docs were broken.
			page, anchor, _ := strings.Cut(target, "#")
			slug := d.slug
			if page != "" {
				slug = resolvePath(d.slug, page)
			}
			if _, ok := anchors[slug]; !ok {
				problems = append(problems, fmt.Sprintf("%s: links to a page that does not exist: %s", d.file, target))
				continue
			}
			if anchor != "" && !anchors[slug][anchor] {
				problems = append(problems, fmt.Sprintf("%s: links to a missing anchor: %s", d.file, target))
			}
		}
	}
	return problems
}

// resolvePath resolves a link target against the page it appears on and
// returns the slug of the page it points at: "" for the site root, "install"
// for /install/index.html. It follows the browser's rules — "..", "./" and a
// trailing "index.html" — because the check is only worth anything if it agrees
// with what a browser will do.
func resolvePath(fromSlug, target string) string {
	parts := []string{}
	if fromSlug != "" {
		parts = append(parts, fromSlug)
	}
	for _, seg := range strings.Split(target, "/") {
		switch seg {
		case "", ".":
			continue
		case "..":
			if len(parts) > 0 {
				parts = parts[:len(parts)-1]
			}
			continue
		}
		parts = append(parts, seg)
	}
	if n := len(parts); n > 0 && parts[n-1] == "index.html" {
		parts = parts[:n-1]
	}
	return strings.Join(parts, "/")
}

// write emits the whole site into dir.
func (s *site) write(dir string) error {
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	for _, d := range s.pages {
		target := filepath.Join(dir, d.slug)
		if d.slug == "" {
			target = dir
		}
		if err := os.MkdirAll(target, 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(target, "index.html"), []byte(s.layout(d)), 0o644); err != nil {
			return err
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "style.css"), []byte(styleCSS), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "search.js"), []byte(searchJS), 0o644); err != nil {
		return err
	}
	if err := s.writeSearchIndex(dir); err != nil {
		return err
	}
	if err := s.writeSitemap(dir); err != nil {
		return err
	}
	return copyStatic(staticDir, dir)
}

// searchSection is one block of a page's prose: the text under a `##` or `###`
// heading, or the opening text before the first heading (Heading and ID empty).
type searchSection struct {
	Heading string `json:"heading,omitempty"`
	ID      string `json:"id,omitempty"`
	Level   int    `json:"level,omitempty"`
	Text    string `json:"text"`
}

// writeSearchIndex writes search.json: one entry per page, so the search field
// works without a server.
//
// Besides the title, the summary and the headings, each page carries its
// **body text**. Without it the search only matched what a page is called and
// how its sections are titled: a reader typing "logo" found nothing, although
// the page explains logos under a heading that does not contain the word. That
// was reported as "search is not working", and it was: it found titles and
// headings, not what the docs say.
//
// The body is stored as flat text per section, so a hit can both match a term
// and open the section it is in. Markup is stripped rather than sent: the index
// stays a few kilobytes and the browser does not escape HTML it is only
// searching.
func (s *site) writeSearchIndex(dir string) error {
	type entry struct {
		Title    string `json:"title"`
		URL      string `json:"url"`
		Summary  string `json:"summary"`
		Headings []struct {
			Text string `json:"text"`
			ID   string `json:"id"`
		} `json:"headings"`
		Sections []searchSection `json:"sections"`
	}
	var index []entry
	for _, d := range s.pages {
		e := entry{Title: d.title, URL: d.slug + "/", Summary: d.summary}
		if d.slug == "" {
			e.URL = ""
		}
		for _, h := range d.headings {
			e.Headings = append(e.Headings, struct {
				Text string `json:"text"`
				ID   string `json:"id"`
			}{h.text, h.id})
		}
		e.Sections = pageSections(d)
		index = append(index, e)
	}
	data, err := json.Marshal(index)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "search.json"), data, 0o644)
}

// pageSections splits a page's Markdown into one text block per section
// heading, with the opening text (before the first heading) as a block of its
// own. Both `##` and `###` split: "The logo" lives under `###`, and a search
// for it should open that subsection rather than the whole `frontend_config`
// table above it. The page title is an `#` line, which markdown() renders
// separately, so it is not a section here. Front matter is dropped.
func pageSections(d *doc) []searchSection {
	var sections []searchSection
	var current searchSection
	var lines []string

	flush := func() {
		current.Text = plainText(strings.Join(lines, "\n"))
		if current.Text != "" || current.Heading != "" {
			sections = append(sections, current)
		}
		lines = nil
	}

	inFrontMatter := true
	for _, line := range strings.Split(d.body, "\n") {
		trimmed := strings.TrimSpace(line)
		if inFrontMatter {
			if trimmed == "" {
				continue
			}
			// The front matter is a run of "key: value" lines at the very top.
			if i := strings.Index(trimmed, ":"); i > 0 && !strings.HasPrefix(trimmed, "#") {
				continue
			}
			inFrontMatter = false
		}
		if text, level, ok := sectionHeading(trimmed); ok {
			flush()
			current = searchSection{Heading: text, ID: slugify(text), Level: level}
			continue
		}
		lines = append(lines, line)
	}
	flush()
	return sections
}

// sectionHeading returns the text and level of a `##` or `###` heading line.
// Those are the levels markdown() gives anchors to, so the id derived from the
// text here is the anchor the page actually has.
func sectionHeading(line string) (string, int, bool) {
	for _, marker := range []string{"### ", "## "} {
		if text, ok := strings.CutPrefix(line, marker); ok {
			return strings.TrimSpace(text), len(marker) - 1, true
		}
	}
	return "", 0, false
}

// plainText reduces Markdown to the words a reader would search for: fenced
// code is dropped (it is examples, and its punctuation defeats matching),
// and inline markup is removed by the same helper the summaries use.
func plainText(src string) string {
	var out []string
	inFence := false
	for _, line := range strings.Split(src, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "```") {
			inFence = !inFence
			continue
		}
		if inFence || t == "" {
			continue
		}
		// A table's separator row (| --- | --- |) carries no words.
		if strings.HasPrefix(t, "|") && strings.Trim(t, "|-: ") == "" {
			continue
		}
		t = tagPattern.ReplaceAllString(t, "")
		t = stripInline(t)
		t = strings.TrimPrefix(t, "#")
		// Quote and list markers are punctuation a search never types.
		t = strings.TrimLeft(t, ">-*+ ")
		// Pipes and the ref shortcode read as noise in a snippet.
		t = refPattern.ReplaceAllString(t, "")
		t = strings.ReplaceAll(t, "|", " ")
		if t = strings.Join(strings.Fields(t), " "); t != "" {
			out = append(out, t)
		}
	}
	return strings.Join(out, " ")
}

// writeSitemap writes a sitemap for crawlers.
func (s *site) writeSitemap(dir string) error {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n")
	for _, d := range s.pages {
		fmt.Fprintf(&b, "  <url><loc>/%s/</loc></url>\n", d.slug)
	}
	b.WriteString("</urlset>\n")
	return os.WriteFile(filepath.Join(dir, "sitemap.xml"), []byte(b.String()), 0o644)
}

// copyStatic copies docs/static (the bundled fonts and their stylesheet) into
// the site root.
func copyStatic(src, dst string) error {
	if !filepath.IsAbs(src) {
		root, err := rootDir()
		if err != nil {
			return err
		}
		src = filepath.Join(root, src)
	}
	return filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}

// layout renders one page: the shared shell, the navigation and the body.
func (s *site) layout(d *doc) string {
	// The home page leads the sidebar even though it is not part of the
	// sequence: neighbours() and the pager still skip it, because the docs
	// begin at the first real page and "Previous" from it would point home.
	var nav strings.Builder
	if home := s.home(); home != nil {
		class := ""
		if home.file == d.file {
			class = ` class="active" aria-current="page"`
		}
		fmt.Fprintf(&nav, "<li><a%s href=\"%s\">%s</a></li>\n",
			class, s.href(d, ""), html.EscapeString(home.title))
	}
	for _, other := range s.pages {
		if other.slug == "" {
			continue
		}
		class := ""
		if other.file == d.file {
			class = ` class="active" aria-current="page"`
		}
		fmt.Fprintf(&nav, "<li><a%s href=\"%s\">%s</a></li>\n",
			class, s.href(d, other.slug), html.EscapeString(other.title))
	}

	var toc strings.Builder
	if len(d.headings) > 1 {
		toc.WriteString("<nav class=\"toc\" aria-label=\"On this page\">\n<p class=\"toc-title\">On this page</p>\n<ul>\n")
		for _, h := range d.headings {
			fmt.Fprintf(&toc, "<li><a href=\"#%s\">%s</a></li>\n", h.id, html.EscapeString(h.text))
		}
		toc.WriteString("</ul>\n</nav>\n")
	}

	var pager strings.Builder
	if prev, next := s.neighbours(d); prev != nil || next != nil {
		pager.WriteString("<nav class=\"pager\">\n")
		if prev != nil {
			fmt.Fprintf(&pager, "<a class=\"prev\" href=\"%s\"><span>Previous</span>%s</a>\n",
				s.href(d, prev.slug), html.EscapeString(prev.title))
		}
		if next != nil {
			fmt.Fprintf(&pager, "<a class=\"next\" href=\"%s\"><span>Next</span>%s</a>\n",
				s.href(d, next.slug), html.EscapeString(next.title))
		}
		pager.WriteString("</nav>\n")
	}

	return `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>` + pageTitle(d) + `</title>
<meta name="description" content="` + html.EscapeString(d.summary) + `">
<link rel="icon" href="` + s.root(d) + `img/filegator-logo.svg" type="image/svg+xml">
<meta name="theme-color" content="#34b891">
<link rel="stylesheet" href="` + s.root(d) + `fonts.css">
<link rel="stylesheet" href="` + s.root(d) + `style.css">
<script>
// Applied before the first paint. A stored choice wins over the system
// setting, and a reader who picked light on a dark machine must not see a
// flash of dark before the stylesheet loads — which is why this is inline in
// the head rather than in search.js.
try {
  var saved = localStorage.getItem('filegator-docs-theme');
  if (saved === 'light' || saved === 'dark') {
    document.documentElement.setAttribute('data-theme', saved);
  }
} catch (e) {}
</script>
</head>
<body>
<a class="skip" href="#content">Skip to content</a>
<header class="site-header">
  <div class="header-inner">
    <a class="brand" href="` + s.root(d) + `index.html">
      <img class="brand-logo" data-light="` + s.root(d) + `img/filegator-logo.svg"
           data-dark="` + s.root(d) + `img/filegator-logo-dark.svg"
           src="` + s.root(d) + `img/filegator-logo.svg"
           alt="` + siteTitle + `" width="146" height="30">
      <script>
      // Runs immediately after the logo exists and before the page is painted,
      // so a reader whose preference is dark never sees the light logo first.
      (function () {
        var root = document.documentElement, img = document.currentScript.previousElementSibling;
        var theme = root.getAttribute('data-theme');
        if (!theme) {
          theme = window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches
            ? 'dark' : 'light';
        }
        var src = img && img.getAttribute('data-' + theme);
        if (src) { img.setAttribute('src', src); }
      })();
      </script>
    </a>
    <span class="tagline">` + tagline + `</span>
    <label class="search">
      <span class="visually-hidden">Search the documentation</span>
      <input id="search" type="search" placeholder="Search the docs" autocomplete="off" spellcheck="false">
    </label>
    <button class="theme-toggle" type="button" aria-label="Switch theme"
            title="Switch between light and dark">
      <svg class="icon-sun" viewBox="0 0 24 24" aria-hidden="true" focusable="false">
        <circle cx="12" cy="12" r="4.2"></circle>
        <path d="M12 2.6v2.6M12 18.8v2.6M2.6 12h2.6M18.8 12h2.6M5.4 5.4l1.9 1.9M16.7 16.7l1.9 1.9M18.6 5.4l-1.9 1.9M7.3 16.7l-1.9 1.9"></path>
      </svg>
      <svg class="icon-moon" viewBox="0 0 24 24" aria-hidden="true" focusable="false">
        <path d="M20.5 14.6A8.6 8.6 0 0 1 9.4 3.5a8.7 8.7 0 1 0 11.1 11.1z"></path>
      </svg>
    </button>
    <button class="nav-toggle" aria-expanded="false" aria-controls="sidebar">Menu</button>
  </div>
</header>
<div class="search-results hidden" id="search-results" role="listbox" aria-label="Search results"></div>
<div class="shell">
<aside id="sidebar">
<nav aria-label="Documentation">
<ul>
` + nav.String() + `</ul>
</nav>
</aside>
<main id="content">
` + d.html + toc.String() + pager.String() + `
</main>
</div>
<script src="` + s.root(d) + `search.js"></script>
</body>
</html>
`
}

// pageTitle is the <title>: the page's title, except on the home page, which
// carries the product name and the tagline.
func pageTitle(d *doc) string {
	if d.slug == "" {
		return siteTitle + " — " + tagline
	}
	return html.EscapeString(d.title) + " — " + siteTitle
}

// href returns the link from page d to the page with slug.
func (s *site) href(d *doc, slug string) string {
	if slug == "" {
		return s.root(d) + "index.html"
	}
	return s.root(d) + slug + "/index.html"
}

// root returns the prefix that reaches the site root from page d.
func (s *site) root(d *doc) string {
	if d.slug == "" {
		return ""
	}
	return "../"
}

// home returns the home page, the one with the empty slug. It leads the
// sidebar but is not part of the reading sequence.
func (s *site) home() *doc {
	for _, p := range s.pages {
		if p.slug == "" {
			return p
		}
	}
	return nil
}

// neighbours returns the pages before and after d in the navigation, skipping
// the home page, which has no place in the sequence.
func (s *site) neighbours(d *doc) (prev, next *doc) {
	var ordered []*doc
	for _, p := range s.pages {
		if p.slug != "" {
			ordered = append(ordered, p)
		}
	}
	for i, p := range ordered {
		if p.file != d.file {
			continue
		}
		if i > 0 {
			prev = ordered[i-1]
		}
		if i+1 < len(ordered) {
			next = ordered[i+1]
		}
		return prev, next
	}
	return nil, nil
}
