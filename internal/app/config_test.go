package app

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/filegator/filegator/internal/config"
)

func TestPublicConfig(t *testing.T) {
	app := newTestApp(t)
	if err := app.config.Update(func(c *config.Config) error {
		c.Frontend.AppName = "My Files"
		c.Frontend.Language = "german"
		c.Frontend.Logo = "/img/logo.svg"
		c.Frontend.UploadMaxSize = 1234
		c.Frontend.GuestRedirection = "https://example.com/login"
		c.Frontend.FilterEntries = []string{".git/", "Thumbs.db"}
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
	got := envelope.Data
	if got.AppName != "My Files" {
		t.Errorf("app_name = %q", got.AppName)
	}
	if got.GuestRedirection != "https://example.com/login" {
		t.Errorf("guest_redirection = %q", got.GuestRedirection)
	}
	if len(got.FilterEntries) != 2 || got.FilterEntries[0] != ".git/" {
		t.Errorf("filter_entries = %v", got.FilterEntries)
	}
	if got.Language != "german" {
		t.Errorf("language = %q", got.Language)
	}
	if got.Logo != "/img/logo.svg" {
		t.Errorf("logo = %q", got.Logo)
	}
	if got.UploadMaxSize != 1234 {
		t.Errorf("upload_max_size = %d", got.UploadMaxSize)
	}
}

// The listing formats "Modified" with the administrator's pattern, so the
// public config must carry it: without it every browser falls back to the
// default and the setting appears to do nothing.
func TestPublicConfigCarriesDateFormat(t *testing.T) {
	app := newTestApp(t)
	rec := get(app, "/api/config")
	var envelope struct {
		Data PublicConfig `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if envelope.Data.DateFormat != "YYYY/MM/DD HH:mm:ss" {
		t.Errorf("date_format = %q, want the default", envelope.Data.DateFormat)
	}

	if err := app.config.Update(func(c *config.Config) error {
		c.Frontend.DateFormat = "DD.MM.YYYY HH:mm"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	rec = get(app, "/api/config")
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if envelope.Data.DateFormat != "DD.MM.YYYY HH:mm" {
		t.Errorf("date_format = %q", envelope.Data.DateFormat)
	}
}

func TestPublicConfigOmitsSecrets(t *testing.T) {
	app := newTestApp(t)
	if err := app.config.Update(func(c *config.Config) error {
		c.Security.SessionKey = "deadbeef"
		c.Security.IPAllowlist = []string{"10.0.0.1"}
		c.Auth.UsersFile = "/secret/users.json"
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	rec := get(app, "/api/config")
	body := rec.Body.String()
	for _, secret := range []string{"deadbeef", "10.0.0.1", "users.json", "session_key", "ip_allowlist"} {
		if strings.Contains(body, secret) {
			t.Errorf("public config leaks %q: %s", secret, body)
		}
	}
}
