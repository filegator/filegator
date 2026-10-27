package app

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/filegator/filegator/internal/config"
)

// The logo is the one piece of branding a deployment is judged by before it is
// configured, so its fallbacks are pinned here: a fresh install must show the
// artwork embedded in the binary (never a URL on the project website, which
// fails offline), a custom logo must be used in both themes when only one
// picture was supplied, and the pair must survive a subpath deployment.

func TestResolveLogoUsesTheEmbeddedArtworkByDefault(t *testing.T) {
	got := resolveLogo(config.Frontend{})
	if got.Logo != config.DefaultLogo {
		t.Errorf("logo = %q, want the embedded %q", got.Logo, config.DefaultLogo)
	}
	if got.LogoDark != config.DefaultLogoDark {
		t.Errorf("dark logo = %q, want the embedded %q", got.LogoDark, config.DefaultLogoDark)
	}
	// A default that is not served by this binary would be a 404 on every
	// page of every fresh install.
	for _, name := range []string{config.DefaultLogo, config.DefaultLogoDark} {
		if !newStaticServer(frontendRoot()).exists(strings.TrimPrefix(name, "/")) {
			t.Errorf("the default logo %s is not an embedded asset", name)
		}
	}
}

func TestResolveLogoKeepsWhatWasConfigured(t *testing.T) {
	got := resolveLogo(config.Frontend{
		Logo:     "  https://example.com/a.svg ",
		LogoDark: "/img/dark.svg",
	})
	if got.Logo != "https://example.com/a.svg" {
		t.Errorf("logo = %q, want the trimmed value", got.Logo)
	}
	if got.LogoDark != "/img/dark.svg" {
		t.Errorf("dark logo = %q", got.LogoDark)
	}
}

// One picture, no dark counterpart: the dark theme shows the same picture
// rather than the built-in dark artwork reappearing beside a custom logo.
func TestResolveLogoFallsBackToTheLightLogo(t *testing.T) {
	got := resolveLogo(config.Frontend{Logo: "/img/mine.svg"})
	if got.LogoDark != "/img/mine.svg" {
		t.Errorf("dark logo = %q, want the light logo; a custom logo must not be blank in dark mode", got.LogoDark)
	}
	if got := resolveLogo(config.Frontend{LogoDark: "/img/mine-dark.svg"}); got.Logo != config.DefaultLogo {
		t.Errorf("logo = %q; only the dark variant was set, so the light one is the built-in", got.Logo)
	}
}

func TestLogoUnderBasePath(t *testing.T) {
	logo := publicLogo{Logo: "/img/a.svg", LogoDark: "https://example.com/b.svg"}
	got := logo.underBasePath("/browser")
	if got.Logo != "/browser/img/a.svg" {
		t.Errorf("logo = %q, want the mount prefix on a local path", got.Logo)
	}
	if got.LogoDark != "https://example.com/b.svg" {
		t.Errorf("dark logo = %q; an absolute URL is not ours to rewrite", got.LogoDark)
	}
	// "//host/x" is a scheme-relative URL, not a path on this server.
	if got := (publicLogo{Logo: "//cdn.example.com/a.svg"}).underBasePath("/x"); got.Logo != "//cdn.example.com/a.svg" {
		t.Errorf("logo = %q; a scheme-relative URL must be left alone", got.Logo)
	}
	if got := logo.underBasePath(""); got.Logo != "/img/a.svg" {
		t.Errorf("logo = %q; the site root has no prefix to add", got.Logo)
	}
}

// The browser needs both variants and cannot tell an empty slot from a
// configured one, so /api/config must never send an empty logo.
func TestPublicConfigCarriesBothLogos(t *testing.T) {
	app := newTestApp(t)
	if err := app.config.Update(func(c *config.Config) error {
		c.Frontend.Logo = ""
		c.Frontend.LogoDark = ""
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	rec := get(app, "/api/config")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var envelope struct {
		Data PublicConfig `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if envelope.Data.Logo != config.DefaultLogo || envelope.Data.LogoDark != config.DefaultLogoDark {
		t.Errorf("logos = %q/%q, want the built-in pair", envelope.Data.Logo, envelope.Data.LogoDark)
	}
}

// Under -base-path the logo URLs are paths on this server, and the browser
// resolves them against the origin rather than the <base href>: without the
// prefix every logo on every page would 404.
func TestPublicConfigLogosFollowTheBasePath(t *testing.T) {
	app := newAppWithBase(t, "/browser")
	rec := get(app, "/browser/api/config")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var envelope struct {
		Data PublicConfig `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if want := "/browser" + config.DefaultLogo; envelope.Data.Logo != want {
		t.Errorf("logo = %q, want %q", envelope.Data.Logo, want)
	}
	if !strings.HasPrefix(envelope.Data.LogoDark, "/browser/") {
		t.Errorf("dark logo = %q, want the mount prefix", envelope.Data.LogoDark)
	}
	// And the prefixed URL is what the server actually answers.
	for _, name := range []string{envelope.Data.Logo, envelope.Data.LogoDark} {
		if rec := get(app, name); rec.Code != http.StatusOK {
			t.Errorf("%s is not served (status %d)", name, rec.Code)
		}
	}
}
