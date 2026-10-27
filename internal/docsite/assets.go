package main

import (
	_ "embed"
	"regexp"
)

// The stylesheet and the search script live in their own files beside this one
// and are compiled in with go:embed. They are not Go string literals because a
// stylesheet full of escaped quotes is unreviewable, and because a real `.js`
// file gets editor support while the site is being built.
var (
	//go:embed style.css
	styleCSS string
	//go:embed search.js
	searchJS string
)

var (
	// hrefPattern finds the links in rendered HTML, for the link check.
	hrefPattern = regexp.MustCompile(`href="([^"]*)"`)
	// idPattern finds the anchors in a page: the ids of every heading, and the
	// id="content" landmark the layout adds.
	idPattern = regexp.MustCompile(`id="([^"]+)"`)
)
