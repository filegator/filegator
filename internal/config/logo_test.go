package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The logo values are written into an <img src> on every page, so what they
// may be is a security boundary as much as a formatting rule: a javascript: or
// data: URL there would run in this origin.

func TestValidateLogoURLAccepts(t *testing.T) {
	for _, ok := range []string{
		"", // the built-in artwork
		"/img/filegator-logo.svg",
		"/some/nested/path.svg",
		"http://example.com/logo.png",
		"https://example.com/logo.svg",
		"https://example.com/logo.svg?v=2",
		"  https://example.com/logo.svg  ", // whitespace is trimmed, not refused
	} {
		if err := ValidateLogoURL(ok); err != nil {
			t.Errorf("ValidateLogoURL(%q) = %v, want accepted", ok, err)
		}
	}
}

func TestValidateLogoURLRejects(t *testing.T) {
	for _, bad := range []string{
		"javascript:alert(1)",
		"JavaScript:alert(1)",
		"data:image/svg+xml,<svg onload=alert(1)>",
		"vbscript:msgbox(1)",
		"//cdn.example.com/a.svg", // scheme-relative: another origin, not a path
		"img/logo.svg",            // relative: resolves against the page, usually 404
		"logo.svg",
		"https://", // no host
		"https://example.com/a.svg\r\nX-Evil: 1",
		"https://example.com/a\x00.svg",
	} {
		if err := ValidateLogoURL(bad); err == nil {
			t.Errorf("ValidateLogoURL(%q) accepted; want refused", bad)
		}
	}
}

// Config.Validate runs the same checks, so a hand-edited config.json cannot
// slip a bad value past the admin API.
func TestConfigValidateChecksEveryLogo(t *testing.T) {
	for _, field := range []string{"logo", "logo_dark"} {
		c := Default()
		if field == "logo" {
			c.Frontend.Logo = "javascript:alert(1)"
		} else {
			c.Frontend.LogoDark = "not a url"
		}
		err := c.Validate()
		if err == nil {
			t.Errorf("%s accepted an invalid value", field)
			continue
		}
		if !strings.Contains(err.Error(), field) {
			t.Errorf("%s: error %q does not name the field", field, err)
		}
	}
}

// A config written by the release that fetched the logo from the project
// website has that URL stored as a value. Loading it must give the embedded
// artwork, not the remote one: the whole point was to stop depending on
// filegator.io.
func TestOldRemoteLogoDefaultIsMigrated(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	data := `{"frontend_config":{"logo":"https://filegator.io/filegator_logo.svg"}}`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Get().Frontend.Logo; got != DefaultLogo {
		t.Errorf("logo = %q, want the embedded %q", got, DefaultLogo)
	}

	// A logo someone typed is theirs, even one on the same host.
	if err := s.Update(func(c *Config) error {
		c.Frontend.Logo = "https://filegator.io/mine.svg"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	reloaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := reloaded.Get().Frontend.Logo; got != "https://filegator.io/mine.svg" {
		t.Errorf("logo = %q; a configured logo must not be rewritten", got)
	}
}

// A config written before logo_dark existed loads with the built-in dark logo
// rather than an empty one: an upgraded install gets the dark assets without
// being asked to configure anything.
func TestLoadOlderConfigGetsTheDarkLogo(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	data := `{"frontend_config":{"logo":"https://example.com/mine.svg"}}`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	got := s.Get().Frontend
	if got.Logo != "https://example.com/mine.svg" {
		t.Errorf("logo = %q, want the configured one", got.Logo)
	}
	if got.LogoDark != DefaultLogoDark {
		t.Errorf("logo_dark = %q, want the default %q", got.LogoDark, DefaultLogoDark)
	}

	// And it is written out with both keys once saved, so an administrator
	// editing the file sees that there is a dark slot.
	if err := s.Update(func(*Config) error { return nil }); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var stored struct {
		Frontend map[string]any `json:"frontend_config"`
	}
	if err := json.Unmarshal(raw, &stored); err != nil {
		t.Fatal(err)
	}
	if _, ok := stored.Frontend["logo_dark"]; !ok {
		t.Error("a saved config does not mention logo_dark")
	}
	if stored.Frontend["logo_dark"] != DefaultLogoDark {
		t.Errorf("saved logo_dark = %v", stored.Frontend["logo_dark"])
	}
}
