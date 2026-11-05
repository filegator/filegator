package app

import (
	"strings"

	"github.com/filegator/filegator/internal/config"
)

// The logo, in two themes.
//
// An administrator sets two URLs: `logo` for light mode and `logo_dark` for
// dark mode. Both are shown in the same places — the file browser header, the
// administration header and the login card — and the theme decides which one a
// visitor sees. Two pictures rather than one because the FileGator wordmark is
// dark grey lettering: on a dark surface it would be unreadable, which is why
// the project ships a lightened variant.
//
// An empty light logo means no logo at all: the header shows the application
// title instead, and the login card shows it above the form. The built-in
// artwork is reachable by setting its path, but it is not substituted for an
// empty field. Every slot is resolved before it is sent to the browser, so the
// interface never has to know where the built-in files live.

// publicLogo is the logo pair /api/config hands the browser.
type publicLogo struct {
	Logo     string `json:"logo"`
	LogoDark string `json:"logo_dark"`
}

// resolveLogo fills in the two URLs from the stored settings.
//
// An empty light logo means no branding image: it is returned as an empty pair
// so the browser can show the title instead. The dark slot is ignored then,
// because the pair is one piece of branding and a value in only one theme would
// leave the other without a logo and without the title that replaces it.
//
// An empty dark logo falls back to the light one: an administrator who supplied
// one picture meant that picture, and showing the built-in FileGator drawing
// beside their own branding in half of the themes would be a surprise. (Dark
// artwork on a dark page may be hard to read; the field exists for exactly that
// case.)
func resolveLogo(f config.Frontend) publicLogo {
	light := strings.TrimSpace(f.Logo)
	if light == "" {
		return publicLogo{}
	}
	dark := strings.TrimSpace(f.LogoDark)
	if dark == "" {
		dark = light
	}
	return publicLogo{Logo: light, LogoDark: dark}
}

// underBasePath prefixes a logo that is a path on this server with the mount
// prefix, so a deployment under /browser loads /browser/img/logo.svg. An
// absolute URL is not ours to rewrite, and "//host/x" is a scheme-relative URL
// rather than a path.
func (l publicLogo) underBasePath(base string) publicLogo {
	if base == "" {
		return l
	}
	prefix := func(logo string) string {
		if strings.HasPrefix(logo, "/") && !strings.HasPrefix(logo, "//") {
			return base + logo
		}
		return logo
	}
	return publicLogo{Logo: prefix(l.Logo), LogoDark: prefix(l.LogoDark)}
}
