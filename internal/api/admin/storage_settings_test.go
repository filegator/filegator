package admin

import (
	"github.com/filegator/filegator/internal/api"

	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/filegator/filegator/internal/auth"
	"github.com/filegator/filegator/internal/config"
)

// storageBody builds a complete storage request from the current state.
func (w *adminTestEnv) storageBody(mutate func(*StorageConfig)) string {
	w.t.Helper()
	rec := w.req(http.MethodGet, "/api/admin/storage", "")
	if rec.Code != http.StatusOK {
		w.t.Fatalf("get storage: status %d", rec.Code)
	}
	var envelope struct {
		Data StorageConfig `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		w.t.Fatal(err)
	}
	cfg := envelope.Data
	if mutate != nil {
		mutate(&cfg)
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		w.t.Fatal(err)
	}
	return string(data)
}

func TestGetStorageConfigDefaultsToLocal(t *testing.T) {
	w := adminEnv(t)
	rec := w.req(http.MethodGet, "/api/admin/storage", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	var result struct {
		Data StorageConfig `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Data.Type != config.StorageTypeLocal {
		t.Errorf("type = %q", result.Data.Type)
	}
	if result.Data.Root != w.root.Dir() {
		t.Errorf("root = %q", result.Data.Root)
	}
}

func TestStorageConfigNeverReturnsPassword(t *testing.T) {
	w := adminEnv(t)
	if err := w.srv.Config.Update(func(c *config.Config) error {
		c.Storage.SFTP.Password = "super-secret-password"
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	rec := w.req(http.MethodGet, "/api/admin/storage", "")
	body := rec.Body.String()
	if containsStr(body, "super-secret-password") {
		t.Errorf("password leaked: %s", body)
	}
	if !containsStr(body, `"sftp_has_password":true`) {
		t.Errorf("has_password flag missing: %s", body)
	}
}

func TestSetStorageConfigSwitchesToLocalFolder(t *testing.T) {
	w := adminEnv(t)
	other := t.TempDir()
	if err := os.MkdirAll(filepath.Join(other, "data"), 0o755); err != nil {
		t.Fatal(err)
	}

	rec := w.submit("/api/admin/storage", w.storageBody(func(c *StorageConfig) {
		c.Type = config.StorageTypeLocal
		c.Root = other
	}))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}

	// The listing must show the new folder immediately.
	listRec := w.req(http.MethodGet, "/api/files?path=/", "")
	var listing api.DirListing
	decode(t, listRec, &listing)
	if len(listing.Entries) != 1 || listing.Entries[0].Name != "data" {
		t.Errorf("entries = %+v", listing.Entries)
	}
	if got := w.srv.Config.Get().Storage.Root; got != other {
		t.Errorf("persisted root = %q", got)
	}
}

func TestSetStorageConfigRejectsUnreachableSFTP(t *testing.T) {
	w := adminEnv(t)
	before := w.srv.StorageDescription()

	rec := w.submit("/api/admin/storage", w.storageBody(func(c *StorageConfig) {
		c.Type = config.StorageTypeSFTP
		c.SFTPHost = "127.0.0.1"
		c.SFTPPort = 1 // nothing listens here
		c.SFTPUser = "demo"
		c.SFTPPassword = "pw"
		c.SFTPTimeout = 1
		c.Root = "/"
	}))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400 (body %s)", rec.Code, rec.Body)
	}
	if after := w.srv.StorageDescription(); after != before {
		t.Errorf("storage changed to %q after a failed switch", after)
	}
}

func TestSetStorageConfigValidatesSFTPFields(t *testing.T) {
	w := adminEnv(t)
	cases := map[string]func(*StorageConfig){
		"missing host": func(c *StorageConfig) {
			c.Type = config.StorageTypeSFTP
			c.SFTPUser = "demo"
			c.SFTPPassword = "pw"
		},
		"missing user": func(c *StorageConfig) {
			c.Type = config.StorageTypeSFTP
			c.SFTPHost = "example.com"
			c.SFTPPassword = "pw"
		},
		"no credentials": func(c *StorageConfig) {
			c.Type = config.StorageTypeSFTP
			c.SFTPHost = "example.com"
			c.SFTPUser = "demo"
		},
		"unknown type": func(c *StorageConfig) { c.Type = "ftp" },
	}
	for name, mutate := range cases {
		rec := w.submit("/api/admin/storage", w.storageBody(mutate))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 400", name, rec.Code)
		}
	}
}

func TestSetStorageConfigKeepsPasswordWhenBlank(t *testing.T) {
	w := adminEnv(t)
	if err := w.srv.Config.Update(func(c *config.Config) error {
		c.Storage.SFTP = config.SFTP{Host: "example.com", User: "demo", Password: "stored"}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	// A blank password with has_password should keep the stored one, so the
	// request gets past config validation (the connection still fails here).
	rec := w.submit("/api/admin/storage", w.storageBody(func(c *StorageConfig) {
		c.Type = config.StorageTypeSFTP
		c.SFTPHost = "127.0.0.1"
		c.SFTPPort = 1
		c.SFTPPassword = ""
		c.SFTPHasPassword = true
		c.SFTPTimeout = 1
		c.Root = "/"
	}))
	// We only assert it was not rejected for a missing password.
	if containsStr(rec.Body.String(), "needs a password") {
		t.Errorf("stored password was not reused: %s", rec.Body)
	}
}

func TestSetStorageConfigRequiresAdmin(t *testing.T) {
	e := newEnv(t)
	e.addUser("bob", "pw", "/bob", auth.RoleUser, auth.PermRead)
	session := e.login("bob", "pw")

	rec := e.do(http.MethodGet, "/api/admin/storage", "", session)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d, want 403", rec.Code)
	}
}
