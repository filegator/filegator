package admin

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/filegator/filegator/internal/config"
)

func TestAuthConfigSQLFields(t *testing.T) {
	w := adminEnv(t)
	if err := w.srv.Config.Update(func(c *config.Config) error {
		c.Auth.SQL = config.SQL{
			DSN:          "user:pw@tcp(localhost:3306)/filegator",
			Table:        "accounts",
			UserCacheTTL: 30,
			MaxOpenConns: 10,
			MaxIdleConns: 5,
			Timeout:      5,
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	rec := w.req(http.MethodGet, "/api/admin/auth", "")
	var result struct {
		Data AuthConfig `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	got := result.Data
	if got.SQLTable != "accounts" || got.SQLUserCacheTTL != 30 {
		t.Errorf("sql fields = %+v", got)
	}
	// The DSN commonly embeds a password, so it must not be echoed back.
	if containsStr(rec.Body.String(), "user:pw@") {
		t.Errorf("DSN with credentials leaked: %s", rec.Body)
	}
	if !got.SQLHasDSN {
		t.Error("sql_has_dsn should be true")
	}
}

func TestSetAuthConfigRejectsSQLWithoutDSN(t *testing.T) {
	w := adminEnv(t)
	before := w.srv.Users.Capabilities()

	for _, typ := range []string{config.AuthTypeMySQL, config.AuthTypePostgres} {
		rec := w.submit("/api/admin/auth", w.authBody(func(c *AuthConfig) {
			c.Type = typ
			c.SQLDSN = ""
		}))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 400", typ, rec.Code)
		}
	}
	if after := w.srv.Users.Capabilities(); after.ManageUsers != before.ManageUsers {
		t.Error("provider changed after a failed switch")
	}
}

func TestSetAuthConfigRejectsUnreachableDatabase(t *testing.T) {
	w := adminEnv(t)
	before := w.srv.Users.Capabilities()

	rec := w.submit("/api/admin/auth", w.authBody(func(c *AuthConfig) {
		c.Type = config.AuthTypeMySQL
		// Nothing listens on port 1.
		c.SQLDSN = "user:pw@tcp(127.0.0.1:1)/filegator"
		c.SQLTimeout = 1
	}))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400 (body %s)", rec.Code, rec.Body)
	}
	if after := w.srv.Users.Capabilities(); after.ManageUsers != before.ManageUsers {
		t.Error("provider changed after a failed switch")
	}
}
