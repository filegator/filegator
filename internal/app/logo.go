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
// Empty means the artwork embedded in this binary. Every slot is resolved
// before it is sent to the browser, so the interface is never unbranded and
// never has to know where the built-in files live.

// publicLogo is the logo pair /api/config hands the browser.
type publicLogo struct {
	Logo     string `json:"logo"`
	LogoDark string `json:"logo_dark"`
}

// resolveLogo fills in the two URLs from the stored settings.
//
// An empty dark logo falls back to the light one when a light logo was set:
// an administrator who supplied one picture meant that picture, and showing
// the built-in FileGator drawing beside their own branding in half of the
// themes would be a surprise. (Dark artwork on a dark page may be hard to read;
// the field exists for exactly that case.)
func resolveLogo(f config.Frontend) publicLogo {
	light := strings.TrimSpace(f.Logo)
	dark := strings.TrimSpace(f.LogoDark)
	if light == "" {
		light = config.DefaultLogo
	}
	switch {
	case dark != "":
		// As configured.
	case strings.TrimSpace(f.Logo) != "":
		dark = light
	default:
		dark = config.DefaultLogoDark
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
