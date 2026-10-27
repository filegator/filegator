package admin

import (
	"bufio"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/filegator/filegator/internal/config"
	"github.com/filegator/filegator/internal/logger"
)

// logEnv wires an admin environment to a temporary event log.
func logEnv(t *testing.T) (*testEnv, string) {
	t.Helper()
	e := adminEnv(t)

	path := filepath.Join(t.TempDir(), "app.log")
	eventLog, err := logger.New(logger.Config{File: path, Level: "debug"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { eventLog.Close() })
	e.srv.Log = eventLog
	return e, path
}

// events reads the log file back.
func events(t *testing.T, path string) []map[string]any {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatal(err)
	}
	defer f.Close()

	var out []map[string]any
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 16*1024), 1<<20)
	for scanner.Scan() {
		var record map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			t.Fatalf("line is not JSON: %q", scanner.Text())
		}
		out = append(out, record)
	}
	return out
}

// findEvent returns the first record with the given event name.
func findEvent(records []map[string]any, name string) map[string]any {
	for _, record := range records {
		if record["event"] == name {
			return record
		}
	}
	return nil
}

func TestLogsUserCreation(t *testing.T) {
	e, path := logEnv(t)

	rec := e.submit("/api/admin/users", `{
		"username":"carol","name":"Carol","role":"user","homedir":"/carol",
		"permissions":["read"],"password":"secret123","create_home":true
	}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}

	record := findEvent(events(t, path), logger.EventAdminUserCreated)
	if record == nil {
		t.Fatal("user creation was not logged")
	}
	// The acting administrator is "user"; the created account is recorded
	// separately, so the two can never be confused when reading the log.
	if record["user"] == "" || record["user"] == nil {
		t.Error("acting administrator missing")
	}
	if record["target_user"] != "carol" {
		t.Errorf("target_user = %v, want carol", record["target_user"])
	}
}

func TestLogsUserUpdateAndDelete(t *testing.T) {
	e, path := logEnv(t)

	e.submit("/api/admin/users", `{
		"username":"carol","name":"Carol","role":"user","homedir":"/carol",
		"permissions":["read"],"password":"secret123","create_home":true
	}`)
	e.submit("/api/admin/users/carol", `{
		"username":"carol","name":"Carol","role":"user","homedir":"/carol",
		"permissions":["read","write"]
	}`)
	if rec := e.submit("/api/admin/users/carol/delete", `{}`); rec.Code != http.StatusNoContent {
		t.Fatalf("delete status %d body %s", rec.Code, rec.Body)
	}

	records := events(t, path)
	if findEvent(records, logger.EventAdminUserUpdated) == nil {
		t.Error("user update was not logged")
	}
	if findEvent(records, logger.EventAdminUserDeleted) == nil {
		t.Error("user deletion was not logged")
	}
}

func TestLogsSettingsSave(t *testing.T) {
	e, path := logEnv(t)

	// Round-trip the current settings so the save is valid.
	// A minimal valid payload: the endpoint replaces the whole settings block,
	// so every required field must be present.
	if rec := e.submit("/api/admin/settings", `{
		"app_name":"FileGator","language":"english","language_selection":true,
		"logo":"","upload_max_size":1048576,"upload_chunk_size":65536,
		"upload_simultaneous":3,"default_archive_name":"archive.zip",
		"session_ttl":3600,"csrf_protection":true
	}`); rec.Code != http.StatusOK {
		t.Fatalf("save settings: status %d body %s", rec.Code, rec.Body)
	}

	if findEvent(events(t, path), logger.EventAdminSettings) == nil {
		t.Fatal("settings save was not logged")
	}
}

func TestGuestCannotBeDeleted(t *testing.T) {
	e := adminEnv(t)

	rec := e.submit("/api/admin/users/guest/delete", `{}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status %d, want 409 so guest access is never silently disabled", rec.Code)
	}

	// The account is still there.
	if _, err := e.srv.Users.Find("guest"); err != nil {
		t.Errorf("guest was removed: %v", err)
	}
}

func TestGuestCannotBeGivenImpossiblePermissions(t *testing.T) {
	e := adminEnv(t)

	rec := e.submit("/api/admin/users/guest", `{
		"username":"guest","name":"Guest","role":"guest","homedir":"/",
		"permissions":["read","download","write"]
	}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400 for a permission a guest can never use", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "write") {
		t.Errorf("message does not name the permission: %s", rec.Body)
	}
}

func TestGuestAcceptsReadAndDownload(t *testing.T) {
	e := adminEnv(t)

	rec := e.submit("/api/admin/users/guest", `{
		"username":"guest","name":"Guest","role":"guest","homedir":"/",
		"permissions":["read","download"]
	}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}
	guest, err := e.srv.Users.Find("guest")
	if err != nil {
		t.Fatal(err)
	}
	if len(guest.Permissions) != 2 {
		t.Errorf("permissions = %v", guest.Permissions)
	}
}

func TestGuestMayBeSwitchedOffByClearingPermissions(t *testing.T) {
	e := adminEnv(t)

	rec := e.submit("/api/admin/users/guest", `{
		"username":"guest","name":"Guest","role":"guest","homedir":"/",
		"permissions":[]
	}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, want an empty permission list to be accepted", rec.Code)
	}
}

func TestPromotingGuestToUserAllowsWrites(t *testing.T) {
	// The restriction is tied to the guest role, not the account, so changing
	// the role back to user restores the normal permission list.
	e := adminEnv(t)

	rec := e.submit("/api/admin/users/guest", `{
		"username":"guest","name":"Guest","role":"user","homedir":"/",
		"permissions":["read","write"]
	}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}
}

func TestGuestBatchDownloadIsAllowed(t *testing.T) {
	// Batch download is a read operation: zipping files the guest may already
	// download one by one grants nothing new, so it is offerable.
	e := adminEnv(t)

	rec := e.submit("/api/admin/users/guest", `{
		"username":"guest","name":"Guest","role":"guest","homedir":"/",
		"permissions":["read","download","batchdownload"]
	}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}
}

func TestGuestStillRefusesWritePermissions(t *testing.T) {
	// Adding batch download must not have widened the list.
	for _, p := range []string{"write", "upload", "zip", "chmod"} {
		e := adminEnv(t)
		rec := e.submit("/api/admin/users/guest", `{
			"username":"guest","name":"Guest","role":"guest","homedir":"/",
			"permissions":["read","`+p+`"]
		}`)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 400", p, rec.Code)
		}
	}
}

func TestGuestRedirectionMustBeAWebURL(t *testing.T) {
	e := adminEnv(t)

	base := func(redir string) string {
		return `{"app_name":"FileGator","language":"english","language_selection":true,
			"logo":"","upload_max_size":1048576,"upload_chunk_size":65536,
			"upload_simultaneous":3,"session_ttl":3600,"csrf_protection":true,
			"guest_redirection":"` + redir + `"}`
	}

	// A script URL would run in this origin when the login page redirects.
	if rec := e.submit("/api/admin/settings", base("javascript:alert(1)")); rec.Code != http.StatusBadRequest {
		t.Errorf("script URL: status %d, want 400", rec.Code)
	}
	// A relative value would redirect the login page to itself.
	if rec := e.submit("/api/admin/settings", base("/login")); rec.Code != http.StatusBadRequest {
		t.Errorf("relative URL: status %d, want 400", rec.Code)
	}
	if rec := e.submit("/api/admin/settings", base("https://example.com/wp-admin/")); rec.Code != http.StatusOK {
		t.Errorf("https URL: status %d body %s", rec.Code, rec.Body)
	}
	if got := e.srv.Config.Get().Frontend.GuestRedirection; got != "https://example.com/wp-admin/" {
		t.Errorf("stored value = %q", got)
	}
}

func TestFrontendDirIsSavedAndValidated(t *testing.T) {
	e := adminEnv(t)
	dir := t.TempDir()

	base := func(frontend string) string {
		return `{"app_name":"FileGator","language":"english","language_selection":true,
			"logo":"","upload_max_size":1048576,"upload_chunk_size":65536,
			"upload_simultaneous":3,"session_ttl":3600,"csrf_protection":true,
			"frontend_dir":"` + frontend + `"}`
	}

	// A missing folder is a typo, not a valid setting: report it now rather than
	// at the next restart, when nobody is looking at the form.
	rec := e.submit("/api/admin/settings", base("/nope/not-here"))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("missing folder: status %d, want 400", rec.Code)
	}

	rec = e.submit("/api/admin/settings", base(dir))
	if rec.Code != http.StatusOK {
		t.Fatalf("valid folder: status %d body %s", rec.Code, rec.Body)
	}
	if got := e.srv.Config.Get().FrontendDir; got != dir {
		t.Errorf("stored frontend_dir = %q, want %q", got, dir)
	}

	// The admin form reads it back.
	settings := e.req(http.MethodGet, "/api/admin/settings", "")
	if settings.Code != http.StatusOK {
		t.Fatalf("get: status %d", settings.Code)
	}
	var body struct {
		Data struct {
			FrontendDir string `json:"frontend_dir"`
		} `json:"data"`
	}
	if err := json.Unmarshal(settings.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Data.FrontendDir != dir {
		t.Errorf("settings report %q", body.Data.FrontendDir)
	}
}

func TestStatusReportsTheFrontendSource(t *testing.T) {
	e := adminEnv(t)
	e.srv.SetFrontendSource("directory")

	rec := e.req(http.MethodGet, "/api/admin/status", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	var body struct {
		Data struct {
			Source string `json:"frontend_source"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Data.Source != "directory" {
		t.Errorf("frontend_source = %q", body.Data.Source)
	}
}

func TestFrontendDirCanBeCleared(t *testing.T) {
	e := adminEnv(t)
	if err := e.srv.Config.Update(func(c *config.Config) error {
		c.FrontendDir = "/tmp"
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	rec := e.submit("/api/admin/settings", `{"app_name":"FileGator","language":"english",
		"language_selection":true,"logo":"","upload_max_size":1048576,"upload_chunk_size":65536,
		"upload_simultaneous":3,"session_ttl":3600,"csrf_protection":true,"frontend_dir":""}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}
	if got := e.srv.Config.Get().FrontendDir; got != "" {
		t.Errorf("frontend_dir = %q, want empty", got)
	}
}
