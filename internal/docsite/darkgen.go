package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// The dark-mode variant of the logo.
//
// The wordmark is dark grey lettering with a green folder. On the site's dark
// background the grey is unreadable. A CSS filter fixes the colour but smears
// the anti-aliased edges, so the file is generated instead: from the published
// artwork, with only the colours changed. No shape is redrawn.
//
// #353535 (the lettering and the folder behind) becomes #e6e6e8, the light
// grey the docs use for body text on a dark surface. The green stays the
// brand's — one of the two greens in the original is folded into the other,
// because two shades three values apart were an accident of the export.
//
// The favicon is not generated here at all. The PNG copies are gone with the
// switch to SVG-only artwork: the site's tab icon is filegator-logo.svg, the
// same drawing in both themes, and nothing produces a second one.

// darkColors maps a colour in the original to its dark-mode equivalent.
var darkColors = map[[3]uint8][3]uint8{
	parseHex("#353535"): parseHex("#e6e6e8"),
	parseHex("#33b88f"): parseHex("#34b891"),
}

// toDarkSVG rewrites colour declarations in an SVG. Only `fill:` and
// `stop-color:` values are touched, so a colour mentioned in a comment or an id
// is left alone, and every shape is preserved exactly.
//
// The gator's eye and nostrils are the exception: they are #353535 drawn on the
// green folder front, and lightening them along with the lettering leaves a
// grey eye on a green gator. darkKeeps names the shapes to leave alone. A
// shape's id follows its style in this artwork, so each declaration is
// attributed to the first id after it.
var darkKeeps = map[string]bool{
	"path80": true, // nostril
	"path82": true, // nostril
	"path88": true, // pupil
}

func toDarkSVG(src string) string {
	decl := regexp.MustCompile(`(?i)(fill|stop-color)\s*:\s*(#[0-9a-f]{6})`)
	idIn := regexp.MustCompile(`\bid="([^"]+)"`)
	var out strings.Builder
	last := 0
	shape := ""
	for _, loc := range decl.FindAllStringSubmatchIndex(src, -1) {
		out.WriteString(src[last:loc[0]])
		last = loc[1]
		if id := idIn.FindStringSubmatch(src[loc[1]:]); id != nil {
			shape = id[1]
		}
		m := src[loc[0]:loc[1]]
		parts := decl.FindStringSubmatch(m)
		if darkKeeps[shape] {
			out.WriteString(m)
			continue
		}
		from := parseHex(parts[2])
		if to, ok := darkColors[from]; ok {
			out.WriteString(parts[1] + ":" + hexString(to))
			continue
		}
		out.WriteString(m)
	}
	out.WriteString(src[last:])
	return out.String()
}

// parseHex reads a #rrggbb string.
func parseHex(s string) [3]uint8 {
	var out [3]uint8
	fmt.Sscanf(strings.TrimPrefix(s, "#"), "%02x%02x%02x", &out[0], &out[1], &out[2])
	return out
}

func hexString(c [3]uint8) string { return fmt.Sprintf("#%02x%02x%02x", c[0], c[1], c[2]) }

// writeDarkFiles writes the dark logo beside the original.
//
// It never touched the favicon, and now does not: the tab icon is
// filegator-logo.svg itself (see the comment above).
func writeDarkFiles(dir string) error {
	svg, err := os.ReadFile(dir + "/filegator-logo.svg")
	if err != nil {
		return err
	}
	return os.WriteFile(dir+"/filegator-logo-dark.svg", []byte(toDarkSVG(string(svg))), 0o644)
}
