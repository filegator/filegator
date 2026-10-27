package api

import (
	"encoding/json"
	"net/http"
	"testing"
)

// TestCapabilitiesReportsJSONProvider checks the flags for the default
// local-FS + JSON-file setup, which supports everything.
func TestCapabilitiesReportsJSONProvider(t *testing.T) {
	e := newEnv(t)

	rec := e.do(http.MethodGet, "/api/capabilities", "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}

	var result struct {
		Data Capabilities `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}

	caps := result.Data
	if !caps.AuthManageUsers {
		t.Error("JSON provider should allow managing users")
	}
	if !caps.AuthChangePassword {
		t.Error("JSON provider should allow changing passwords")
	}
	if !caps.StorageChmod {
		t.Error("local storage should support chmod")
	}
	if !caps.StorageServerBrowse {
		t.Error("local storage should allow browsing the host filesystem")
	}
}

// TestCapabilitiesIsPublic checks that the endpoint needs no session, since the
// frontend reads it before deciding which controls to show.
func TestCapabilitiesIsPublic(t *testing.T) {
	e := newEnv(t)
	rec := e.do(http.MethodGet, "/api/capabilities", "", "")
	if rec.Code != http.StatusUnauthorized && rec.Code != http.StatusOK {
		t.Fatalf("status %d, want 200 (public)", rec.Code)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("capabilities must be public, got %d", rec.Code)
	}
}
