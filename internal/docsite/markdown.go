package main

import (
	"fmt"
	"html"
	"regexp"
	"strings"
	"unicode/utf8"
)

// markdown renders the Markdown the docs use.
//
// This is a renderer for *this* documentation, not a general Markdown library.
// It understands exactly the constructs the pages in docs/content use, and it
// grows a construct at a time, with the page that needs it. The alternative —
// a full CommonMark implementation, or a dependency for one — was rejected: the
// project is standard library only, and the docs do not need reference links or
// lists nested inside block quotes.
//
// The supported set:
//
//   - headings (# … ######), with github-style ids
//   - paragraphs, joined across soft line breaks
//   - fenced code blocks, language kept as a class
//   - pipe tables with a header row and column alignment
//   - block quotes (one level)
//   - unordered lists ("- ", "* ") and ordered lists ("1. "), nested to any
//     depth, with continuation lines
//   - horizontal rules
//   - inline: code spans, links (including the {{< ref >}} form), bold, italic,
//     inline HTML, HTML comments dropped
//
// Anything else is emitted as text, so a construct this does not know shows up
// on the page instead of disappearing. `make docs-check` verifies the result.
func markdown(src string, page *doc) string {
	var out strings.Builder
	lines := strings.Split(src, "\n")
	i := 0
	for i < len(lines) {
		line := lines[i]

		switch {
		case markerOnly(line):
			// An HTML comment line carries nothing to show.
			i++
			continue

		case strings.HasPrefix(line, "```"):
			i = fence(lines, i, &out)
			continue

		case strings.HasPrefix(line, "|") && i+1 < len(lines) && isTableRule(lines[i+1]):
			i = table(lines, i, &out, page)
			continue

		case strings.HasPrefix(line, "#"):
			level := len(line) - len(strings.TrimLeft(line, "#"))
			text := strings.TrimSpace(strings.TrimLeft(line, "#"))
			if level > 6 {
				level = 6
			}
			fmt.Fprintf(&out, "<h%d id=\"%s\">%s</h%d>\n",
				level, slugify(stripInline(text)), inline(text, page), level)
			i++
			continue

		case strings.HasPrefix(line, ">"):
			var quote []string
			for i < len(lines) && strings.HasPrefix(lines[i], ">") {
				quote = append(quote, strings.TrimPrefix(strings.TrimPrefix(lines[i], ">"), " "))
				i++
			}
			out.WriteString("<blockquote>\n" + markdown(strings.Join(quote, "\n"), page) + "</blockquote>\n")
			continue

		case isListLine(line):
			_, ordered := listMarker(line, true)
			list, next := listItems(lines, i, page, ordered)
			out.WriteString(list)
			i = next
			continue

		case line == "---" || line == "***":
			out.WriteString("<hr>\n")
			i++
			continue

		case strings.TrimSpace(line) == "":
			i++
			continue
		}

		// A paragraph runs until a blank line or the start of a block.
		var para []string
		for i < len(lines) && strings.TrimSpace(lines[i]) != "" && !startsBlock(lines[i]) {
			if !markerOnly(lines[i]) {
				para = append(para, strings.TrimSpace(lines[i]))
			}
			i++
		}
		if len(para) > 0 {
			out.WriteString("<p>" + inline(strings.Join(para, "\n"), page) + "</p>\n")
		}
	}
	return out.String()
}

// fence renders a fenced code block starting at line i and returns the line
// after the closing fence.
func fence(lines []string, i int, out *strings.Builder) int {
	lang := strings.TrimSpace(strings.TrimPrefix(lines[i], "```"))
	i++
	var code []string
	for i < len(lines) && !strings.HasPrefix(lines[i], "```") {
		code = append(code, lines[i])
		i++
	}
	i++ // closing fence
	class := ""
	if lang != "" {
		class = ` class="language-` + html.EscapeString(lang) + `"`
	}
	out.WriteString("<pre><code" + class + ">" + html.EscapeString(strings.Join(code, "\n")) + "\n</code></pre>\n")
	return i
}

// markerOnly reports whether a line is an HTML comment and nothing else. Such a
// line carries no text, so it must not break a table or a list away from the
// paragraph that introduces it — the endpoint tables in api.md sit directly
// under a `<!-- BEGIN apidoc:… -->` marker.
func markerOnly(line string) bool {
	t := strings.TrimSpace(line)
	return strings.HasPrefix(t, "<!--") && strings.HasSuffix(t, "-->")
}

// listMarker reports whether a line starts a list item of the kind ordered
// asks for, and returns the line with its marker removed. Bullets and numbers
// are two kinds rather than one: a list that switches from "- " to "1. "
// starts a new list.
func listMarker(line string, ordered bool) (string, bool) {
	t := strings.TrimLeft(line, " ")
	if !ordered {
		if strings.HasPrefix(t, "- ") || strings.HasPrefix(t, "* ") {
			return t[2:], true
		}
		return "", false
	}
	digits := 0
	for digits < len(t) && t[digits] >= '0' && t[digits] <= '9' {
		digits++
	}
	// One or two digits, then ". ": "2026. was a good year" is a sentence.
	if digits == 0 || digits > 2 || !strings.HasPrefix(t[digits:], ". ") {
		return "", false
	}
	return t[digits+2:], true
}

// isListLine reports whether a line starts a list item of either kind.
func isListLine(line string) bool {
	_, bullet := listMarker(line, false)
	_, numbered := listMarker(line, true)
	return bullet || numbered
}

// isBullet reports whether a line starts an unordered list item.
func isBullet(line string) bool {
	_, ok := listMarker(line, false)
	return ok
}

// startsBlock reports whether a line begins a block markdown() handles, which
// ends the paragraph before it.
func startsBlock(line string) bool {
	if markerOnly(line) || isListLine(line) {
		return true
	}
	return strings.HasPrefix(line, "```") || strings.HasPrefix(line, "#") ||
		strings.HasPrefix(line, ">") || line == "---"
}

func isTableRule(line string) bool {
	t := strings.TrimSpace(line)
	return strings.HasPrefix(t, "|") && strings.Contains(t, "-") &&
		strings.Trim(t, "|-: ") == ""
}

// table renders a pipe table, taking column alignment from the rule row.
func table(lines []string, i int, out *strings.Builder, page *doc) int {
	cells := func(line string) []string {
		parts := strings.Split(strings.Trim(strings.TrimSpace(line), "|"), "|")
		for k := range parts {
			parts[k] = strings.TrimSpace(parts[k])
		}
		return parts
	}
	var align []string
	for _, c := range cells(lines[i+1]) {
		switch {
		case strings.HasPrefix(c, ":") && strings.HasSuffix(c, ":"):
			align = append(align, "center")
		case strings.HasSuffix(c, ":"):
			align = append(align, "right")
		default:
			align = append(align, "")
		}
	}
	attr := func(k int) string {
		if k < len(align) && align[k] != "" {
			return ` style="text-align:` + align[k] + `"`
		}
		return ""
	}

	header := cells(lines[i])
	i += 2 // header and rule
	out.WriteString("<div class=\"table-wrap\">\n<table>\n<thead><tr>")
	for k, c := range header {
		out.WriteString("<th" + attr(k) + ">" + inline(c, page) + "</th>")
	}
	out.WriteString("</tr></thead>\n<tbody>\n")
	for i < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[i]), "|") {
		out.WriteString("<tr>")
		for k, c := range cells(lines[i]) {
			out.WriteString("<td" + attr(k) + ">" + inline(c, page) + "</td>")
		}
		out.WriteString("</tr>\n")
		i++
	}
	out.WriteString("</tbody>\n</table>\n</div>\n")
	return i
}

// listItems renders a list starting at line i and everything belonging to it.
// ordered selects the kind: "1. " items become <ol>, "- " and "* " items
// become <ul>.
//
// Indentation decides nesting: a marker indented two or more spaces past the
// list's own indent starts a nested list inside the current item, which is how
// the docs write "a heading, then the sub-points that explain it". The item's
// children are rendered first and attached to it, so the nesting appears in the
// markup where the author wrote it.
func listItems(lines []string, i int, page *doc, ordered bool) (string, int) {
	base := indentOf(lines[i])
	tag := "ul"
	if ordered {
		tag = "ol"
	}

	type item struct {
		text  string
		parts []string // nested lists
	}
	var (
		out   strings.Builder
		items []item
	)

	flush := func() {
		if len(items) == 0 {
			return
		}
		it := items[len(items)-1]
		items = items[:len(items)-1]
		out.WriteString("<li>" + inline(strings.TrimSpace(it.text), page))
		for _, part := range it.parts {
			out.WriteString(part)
		}
		out.WriteString("</li>\n")
	}

	out.WriteString("<" + tag + ">\n")
scan:
	for i < len(lines) {
		line := lines[i]

		if strings.TrimSpace(line) == "" {
			i++
			// A blank line ends the list unless the next line continues it at
			// this indent or deeper.
			if i >= len(lines) || !isListLine(lines[i]) || indentOf(lines[i]) < base-1 {
				break
			}
			continue
		}

		indent := indentOf(line)
		switch {
		case !isListLine(line):
			// A continuation line, indented under the item it explains.
			if indent >= base+2 && len(items) > 0 {
				items[len(items)-1].text += " " + strings.TrimSpace(line)
				i++
				continue
			}
			break scan
		case indent >= base+2:
			// Nested, and of whatever kind its own marker is.
			_, nestedOrdered := listMarker(line, true)
			sub, next := listItems(lines, i, page, nestedOrdered)
			if len(items) > 0 {
				items[len(items)-1].parts = append(items[len(items)-1].parts, sub)
			} else {
				out.WriteString("<li>" + sub + "</li>\n")
			}
			i = next
		default:
			text, sameKind := listMarker(line, ordered)
			if indent < base || !sameKind {
				// Dedented, or a marker of the other kind: this list is over
				// and the caller starts the next one at this line.
				break scan
			}
			flush()
			items = append(items, item{text: text})
			i++
		}
	}
	for len(items) > 0 {
		flush()
	}
	return out.String() + "</" + tag + ">\n", i
}

// indentOf returns the number of leading spaces.
func indentOf(line string) int {
	return len(line) - len(strings.TrimLeft(line, " "))
}

// inline renders inline markup.
//
// It works left to right in one pass rather than by substituting placeholders.
// Placeholders are the obvious approach and they were wrong: a placeholder is
// itself matched by the later steps, so the number standing for a held link
// ended up inside that link's href. Scanning avoids the whole class of bug —
// the text a rule inserts is never rescanned.
func inline(text string, page *doc) string {
	var out strings.Builder
	for i := 0; i < len(text); {
		rest := text[i:]

		// Code span: contents are literal.
		if rest[0] == '`' {
			if end := strings.IndexByte(rest[1:], '`'); end >= 0 {
				out.WriteString("<code>" + html.EscapeString(rest[1:end+1]) + "</code>")
				i += end + 2
				continue
			}
		}

		// A bare {{< ref "page.md" >}}, with an optional anchor.
		if strings.HasPrefix(rest, "{{<") {
			if m := refPattern.FindStringSubmatch(rest); m != nil {
				out.WriteString(`<a href="` + linkTarget(m[1], m[2], page) + `">`)
				i += len(m[0])
				continue
			}
		}

		// [label](target), including the docs' [label]({{< ref "page.md" >}})
		// form and a plain relative "page.md" link. All become a link to the
		// page's URL; anything else (an http URL, an anchor) is left as is.
		if rest[0] == '[' {
			if m := refLinkPattern.FindStringSubmatch(rest); m != nil {
				out.WriteString(`<a href="` + linkTarget(m[2], m[3], page) + `">` + inline(m[1], page) + `</a>`)
				i += len(m[0])
				continue
			}
			if m := linkPattern.FindStringSubmatch(rest); m != nil && strings.HasPrefix(rest, m[0]) {
				out.WriteString(`<a href="` + linkHref(m[2], page) + `">` + inline(m[1], page) + `</a>`)
				i += len(m[0])
				continue
			}
		}

		// HTML comment: nothing to show.
		if strings.HasPrefix(rest, "<!--") {
			if end := strings.Index(rest, "-->"); end >= 0 {
				i += end + 3
				continue
			}
		}

		// An autolink: <https://example.com>. Rendered as a link, which is what
		// a reader copying the docs expects; without this the URL vanished from
		// the page, because the angle brackets looked like an HTML tag.
		if rest[0] == '<' {
			if m := autolinkPattern.FindStringSubmatch(rest); m != nil {
				out.WriteString(`<a href="` + m[1] + `">` + m[1] + `</a>`)
				i += len(m[0])
				continue
			}
		}

		// Other inline HTML the docs carry, kept as markup.
		if rest[0] == '<' {
			if m := tagPattern.FindString(rest); m != "" && strings.HasPrefix(rest, m) {
				out.WriteString(m)
				i += len(m)
				continue
			}
		}

		// **bold** and *italic*.
		if strings.HasPrefix(rest, "**") {
			if end := strings.Index(rest[2:], "**"); end >= 0 {
				out.WriteString("<strong>" + inline(rest[2:2+end], page) + "</strong>")
				i += end + 4
				continue
			}
		}
		if rest[0] == '*' {
			if end := strings.IndexByte(rest[1:], '*'); end >= 0 && end > 0 {
				out.WriteString("<em>" + inline(rest[1:end+1], page) + "</em>")
				i += end + 2
				continue
			}
		}

		// Anything else is text. Escape one rune and move on.
		_, size := utf8.DecodeRuneInString(rest)
		out.WriteString(html.EscapeString(rest[:size]))
		i += size
	}
	return out.String()
}

var (
	// refPattern matches the ref shortcode with or without an anchor.
	refPattern = regexp.MustCompile(`\{\{<\s*ref\s+"([^"]+)"\s*>\}\}(#[a-z0-9-]*)?`)
	// tagPattern matches an inline HTML tag.
	tagPattern = regexp.MustCompile(`</?[a-zA-Z][^>]*>`)
	// autolinkPattern matches <https://…> and <mailto:…>.
	autolinkPattern = regexp.MustCompile(`^<((?:https?|mailto):[^>\s]+)>`)
	// linkPattern matches [label](target).
	linkPattern = regexp.MustCompile(`^\[([^\]]+)\]\(([^)\s]+)\)`)
	// refLinkPattern matches [label]({{< ref "page.md" >}}#anchor).
	refLinkPattern = regexp.MustCompile(`^\[([^\]]+)\]\(\{\{<\s*ref\s+"([^"]+)"\s*>\}\}(#[a-z0-9-]*)?\)`)
	// mdLinkPattern matches a relative link to another page.
	mdLinkPattern = regexp.MustCompile(`^([a-z0-9_-]+)\.md(#.*)?$`)
	// The three used by stripInline to remove markup from slugs and summaries.
	inlineCodePattern = regexp.MustCompile("`([^`]+)`")
	linkTextPattern   = regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`)
)

// linkTarget returns the URL of a page, relative to the page it is linked
// from, plus an optional anchor.
//
// The result is a real relative path: a page in a subfolder links back to the
// site root with "../". Emitting a bare "storage/index.html" from
// /install/index.html would point at /install/storage/, which is exactly the
// broken-link bug this signature exists to prevent.
func linkTarget(file, anchor string, from *doc) string {
	return upTo(from) + pageURL(file) + anchor
}

// linkHref turns a link target into a URL. A relative .md file becomes the
// page's URL; anything else (an http URL, an anchor) is left as written.
func linkHref(target string, from *doc) string {
	if m := mdLinkPattern.FindStringSubmatch(target); m != nil {
		return upTo(from) + pageURL(m[1]+".md") + m[2]
	}
	return target
}

// pageURL is the site path of a page, relative to the site root:
// "_index.md" is the root itself, and every other page is <slug>/index.html.
func pageURL(file string) string {
	slug := strings.TrimSuffix(file, ".md")
	if slug == "_index" || slug == "" {
		return "index.html"
	}
	return slug + "/index.html"
}

// upTo is the prefix that reaches the site root from a page: "" at the root,
// "../" inside a subfolder. Every page is one folder deep, so this is a
// constant per page rather than a computed path.
func upTo(d *doc) string {
	if d == nil || d.slug == "" {
		return ""
	}
	return "../"
}

// stripInline removes markup for use in an id or a summary. The order matters:
// bold is stripped in its own pass, because `\*\*?([^*]+)\*\*?` cannot tell
// "**a**" from "*b*" when both appear, and left the asterisks behind.
func stripInline(s string) string {
	s = inlineCodePattern.ReplaceAllString(s, "$1")
	s = linkTextPattern.ReplaceAllString(s, "$1")
	s = strings.ReplaceAll(s, "**", "")
	s = strings.ReplaceAll(s, "*", "")
	return strings.TrimSpace(s)
}

// slugify turns heading text into a github-style anchor: lowercase, spaces
// become hyphens, and everything else that is not a letter or digit is dropped.
func slugify(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '-' || r == '_':
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-")
}
