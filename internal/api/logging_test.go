package api

import (
	"bufio"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/filegator/filegator/internal/auth"
	"github.com/filegator/filegator/internal/config"
	"github.com/filegator/filegator/internal/logger"
	"github.com/filegator/filegator/internal/security"
)

// logEnv returns an environment whose server writes events to a temp file.
//
// The file is returned so a test can read records back.
func logEnv(t *testing.T) (*env, string) {
	t.Helper()
	e := newEnv(t)

	path := filepath.Join(t.TempDir(), "app.log")
	eventLog, err := logger.New(logger.Config{File: path, Level: "debug"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { eventLog.Close() })
	e.srv.Log = eventLog
	return e, path
}

// events reads the log file back as decoded records.
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

func TestLogsSuccessfulLogin(t *testing.T) {
	e, path := logEnv(t)
	e.login("admin", "admin123")

	record := findEvent(events(t, path), logger.EventLogin)
	if record == nil {
		t.Fatal("successful login was not logged")
	}
	if record["user"] != "admin" {
		t.Errorf("user = %v", record["user"])
	}
	if record["ip"] != "127.0.0.1" {
		t.Errorf("ip = %v", record["ip"])
	}
}

func TestLogsFailedLogin(t *testing.T) {
	e, path := logEnv(t)
	e.do(http.MethodPost, "/api/auth/login", `{"username":"admin","password":"wrong"}`, "")

	record := findEvent(events(t, path), logger.EventLoginFailed)
	if record == nil {
		t.Fatal("failed login was not logged")
	}
	if record["level"] != "WARN" {
		t.Errorf("level = %v, want WARN", record["level"])
	}
	if record["user"] != "admin" {
		t.Errorf("user = %v, want the attempted account", record["user"])
	}
	// The password must never reach the log.
	if _, present := record["password"]; present {
		t.Error("password field was logged")
	}
}

func TestLogsLockout(t *testing.T) {
	e, path := logEnv(t)
	// The environment allows three attempts.
	for i := 0; i < 4; i++ {
		e.do(http.MethodPost, "/api/auth/login", `{"username":"admin","password":"wrong"}`, "")
	}

	if findEvent(events(t, path), logger.EventLoginLocked) == nil {
		t.Fatal("lockout was not logged")
	}
}

func TestLogsLogout(t *testing.T) {
	e, path := logEnv(t)
	session := e.login("admin", "admin123")
	e.do(http.MethodPost, "/api/auth/logout", "", session)

	record := findEvent(events(t, path), logger.EventLogout)
	if record == nil || record["user"] != "admin" {
		t.Fatalf("record = %v", record)
	}
}

func TestLogsPermissionDenied(t *testing.T) {
	e, path := logEnv(t)
	e.addUser("reader", "pw", "/alice", auth.RoleUser, auth.PermRead)
	session := e.login("reader", "pw")

	e.do(http.MethodPost, "/api/files/delete", `{"paths":["/notes.md"]}`, session)

	record := findEvent(events(t, path), logger.EventPermissionDenied)
	if record == nil {
		t.Fatal("permission denial was not logged")
	}
	if record["user"] != "reader" {
		t.Errorf("user = %v", record["user"])
	}
}

func TestLogsFileOperations(t *testing.T) {
	e, path := logEnv(t)
	e.addUser("writer", "pw", "/alice", auth.RoleUser,
		auth.PermRead, auth.PermWrite, auth.PermDownload)
	session := e.login("writer", "pw")

	e.do(http.MethodPost, "/api/files/create", `{"parent":"/","name":"new.txt"}`, session)
	e.do(http.MethodPost, "/api/files/save", `{"path":"/notes.md","content":"changed"}`, session)
	e.do(http.MethodPost, "/api/files/rename", `{"from":"/new.txt","to":"/renamed.txt"}`, session)
	e.do(http.MethodPost, "/api/files/delete", `{"paths":["/renamed.txt"]}`, session)

	records := events(t, path)
	for _, name := range []string{
		logger.EventFileCreated, logger.EventFileSaved,
		logger.EventFileRenamed, logger.EventFileDeleted,
	} {
		record := findEvent(records, name)
		if record == nil {
			t.Errorf("%s was not logged", name)
			continue
		}
		if record["user"] != "writer" {
			t.Errorf("%s: user = %v", name, record["user"])
		}
	}
}

func TestLogsPasswordChange(t *testing.T) {
	e, path := logEnv(t)
	e.addUser("user1", "pw", "/alice", auth.RoleUser, auth.PermRead)
	session := e.login("user1", "pw")

	// A wrong current password is the interesting case: it is what an attacker
	// holding a stolen session would produce.
	e.do(http.MethodPost, "/api/auth/password",
		`{"current_password":"nope","new_password":"newpw123456"}`, session)
	record := findEvent(events(t, path), logger.EventPasswordChanged)
	if record == nil {
		t.Fatal("refused password change was not logged")
	}
}

func TestLogsCSRFFailure(t *testing.T) {
	e, path := logEnv(t)
	session := e.login("admin", "admin123")

	// Turn CSRF on: the environment leaves it off for simpler tests.
	if err := e.srv.Config.Update(func(c *config.Config) error {
		c.Security.CSRFProtection = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	// A state-changing request carrying the session but no CSRF header must
	// be refused and logged.
	r := httptest.NewRequest(http.MethodPost, "/api/files/delete",
		strings.NewReader(`{"paths":["/top.txt"]}`))
	r.RemoteAddr = "127.0.0.1:12345"
	r.AddCookie(&http.Cookie{Name: security.CookieName, Value: session})
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, r)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d, want 403", rec.Code)
	}
	if findEvent(events(t, path), logger.EventCSRFFailed) == nil {
		t.Fatal("CSRF failure was not logged")
	}
}
