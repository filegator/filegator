package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Docs search was reported as "I type logo and nothing happens". It was broken
// twice over, and both failures were invisible to the existing tests: they
// checked that search.json has an entry per page, not what it contains or
// whether the script can reach it. These tests are about what a reader gets
// back when they type.

type searchIndexEntry struct {
	Title    string `json:"title"`
	URL      string `json:"url"`
	Summary  string `json:"summary"`
	Headings []struct {
		Text string `json:"text"`
		ID   string `json:"id"`
	} `json:"headings"`
	Sections []struct {
		Heading string `json:"heading"`
		ID      string `json:"id"`
		Level   int    `json:"level"`
		Text    string `json:"text"`
	} `json:"sections"`
}

// buildIndex writes the site to a temp dir and reads search.json back.
func buildIndex(t *testing.T) []searchIndexEntry {
	t.Helper()
	s, err := build(repoContent)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	out := t.TempDir()
	if err := s.write(out); err != nil {
		t.Fatalf("write: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(out, "search.json"))
	if err != nil {
		t.Fatal(err)
	}
	var index []searchIndexEntry
	if err := json.Unmarshal(data, &index); err != nil {
		t.Fatalf("search.json is not valid JSON: %v", err)
	}
	return index
}

// A word that appears only in a page's prose must be findable. Before the body
// was indexed, "logo" matched nothing at all, because no title, summary or
// heading says it — the page explains it under a heading called something else.
func TestSearchIndexContainsThePageProse(t *testing.T) {
	index := buildIndex(t)
	var withLogo []string
	for _, page := range index {
		for _, s := range page.Sections {
			if strings.Contains(strings.ToLower(s.Text), "logo") {
				withLogo = append(withLogo, page.Title+" #"+s.ID)
				break
			}
		}
	}
	if len(withLogo) == 0 {
		t.Fatal(`no page's body text contains "logo"; the index carries titles and headings only, which is the reported "search finds nothing"`)
	}
	// The page that documents the setting must be among them.
	found := false
	for _, hit := range withLogo {
		if strings.HasPrefix(hit, "Configuration") {
			found = true
		}
	}
	if !found {
		t.Errorf("configuration has no indexable text about logos; found %v", withLogo)
	}
}

// headingIDs returns every `id="…"` on an h2/h3 in rendered HTML.
func headingIDs(html string) []string {
	re := regexp.MustCompile(`<h[23] id="([^"]+)"`)
	var ids []string
	for _, m := range re.FindAllStringSubmatch(html, -1) {
		ids = append(ids, m[1])
	}
	return ids
}

// A section anchor must be the one the page really has, or a search result
// links to a place that does not exist. "The logo" is an h3, whose anchor is
// its own — not its parent's.
func TestSearchSectionsAnchorToRealHeadings(t *testing.T) {
	s, err := build(repoContent)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	for _, d := range s.pages {
		anchors := map[string]bool{}
		for _, h := range headingIDs(d.html) {
			anchors[h] = true
		}
		for _, section := range pageSections(d) {
			if section.ID == "" {
				continue
			}
			if !anchors[section.ID] {
				t.Errorf("%s: index names anchor %q, which the page does not have", d.file, section.ID)
			}
			if section.Heading == "" {
				t.Errorf("%s: section with anchor %q has no heading text", d.file, section.ID)
			}
			if section.Level != 2 && section.Level != 3 {
				t.Errorf("%s: section %q has level %d; only h2/h3 carry anchors", d.file, section.ID, section.Level)
			}
		}
	}
}

// A subsection is its own block: "The logo" is an h3 under
// "frontend_config", and a hit there must land on the subsection rather than
// at the top of the table above it.
func TestNestedHeadingIsItsOwnSection(t *testing.T) {
	s, err := build(repoContent)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	var page *doc
	for _, d := range s.pages {
		if d.file == "configuration.md" {
			page = d
		}
	}
	if page == nil {
		t.Fatal("configuration.md is missing")
	}
	var logo *searchSection
	sections := pageSections(page)
	for i := range sections {
		if sections[i].Heading == "The logo" {
			logo = &sections[i]
			break
		}
	}
	if logo == nil {
		t.Fatal(`no "The logo" section in the index; it must not be folded into the section above it`)
	}
	if logo.ID != "the-logo" || logo.Level != 3 {
		t.Errorf("section = id %q level %d, want the-logo at h3", logo.ID, logo.Level)
	}
	if !strings.Contains(strings.ToLower(logo.Text), "dark") {
		t.Errorf(`"The logo" section does not carry its own prose: %q`, cut(logo.Text, 120))
	}
	// And it must not also be repeated inside its parent.
	for _, section := range sections {
		if section.Heading == "`frontend_config`" && strings.Contains(strings.ToLower(section.Text), "logo url") {
			t.Error("the parent section still contains the subsection's text")
		}
	}
}

// The script had two variables called `root`: the theme code reassigned the
// site-root URL to <html>, so the index was fetched from
// "/page/[object HTMLHtmlElement]search.json". The answer was an error page,
// an empty index, and "No page matches that" for every query.
func TestSearchScriptHasNoShadowedRoot(t *testing.T) {
	src, err := os.ReadFile("search.js")
	if err != nil {
		t.Fatal(err)
	}
	code := string(src)
	if strings.Contains(code, "var root =") {
		t.Error("search.js declares `root` twice again (site root and document element); use distinct names")
	}
	if !strings.Contains(code, "siteRoot + 'search.json'") {
		t.Error("search.js no longer fetches the index from the site root")
	}
	// The index URL must not be built from the document element.
	if strings.Contains(code, "document.documentElement + ") {
		t.Error("search.js builds a URL from document.documentElement")
	}
}

// Fenced code is examples, and its punctuation defeats matching: a reader
// searching for a shell command's words should find the prose that explains
// it, not a wall of symbols.
func TestPlainTextDropsCodeAndMarkup(t *testing.T) {
	src := "Intro with `code` and [a link](../x.md).\n\n```sh\nmake test\n```\n\n- item **bold**\n"
	got := plainText(src)
	for _, want := range []string{"Intro with code", "a link", "item bold"} {
		if !strings.Contains(got, want) {
			t.Errorf("plainText lost %q: %q", want, got)
		}
	}
	for _, unwanted := range []string{"make test", "```", "**", "]("} {
		if strings.Contains(got, unwanted) {
			t.Errorf("plainText kept %q: %q", unwanted, got)
		}
	}
}

func cut(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
