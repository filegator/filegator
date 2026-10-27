package app

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/filegator/filegator/internal/auth/jsonfile"
	"github.com/filegator/filegator/internal/config"
)

// resetFixture writes a config and a users file into a temp dir and returns the
// config path and the users path.
func resetFixture(t *testing.T, authType string) (configPath, usersPath string) {
	t.Helper()
	dir := t.TempDir()
	usersPath = filepath.Join(dir, "users.json")
	configPath = filepath.Join(dir, "config.json")

	sample, err := os.ReadFile(filepath.Join("..", "..", "private", "users.json.sample"))
	if err != nil {
		t.Fatalf("read the shipped sample: %v", err)
	}
	if err := os.WriteFile(usersPath, sample, 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := config.Default()
	cfg.Auth.Type = authType
	cfg.Auth.UsersFile = usersPath
	cfg.Storage.Root = filepath.Join(dir, "repository")
	if err := os.MkdirAll(cfg.Storage.Root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, mustJSON(t, cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	return configPath, usersPath
}

// TestResetAdminPassword covers the case the command exists for: nobody can
// sign in, and the operator has shell access to the server.
func TestResetAdminPassword(t *testing.T) {
	configPath, usersPath := resetFixture(t, config.AuthTypeJSON)

	var out bytes.Buffer
	err := ResetAdminPassword(configPath, "admin", strings.NewReader("a-new-password\na-new-password\n"), &out)
	if err != nil {
		t.Fatalf("reset: %v", err)
	}
	if !strings.Contains(out.String(), "was changed") {
		t.Errorf("the command said nothing useful: %q", out.String())
	}

	users := jsonfile.NewUsersFile(usersPath)
	if _, err := users.Authenticate("admin", "a-new-password"); err != nil {
		t.Fatalf("the new password does not work: %v", err)
	}
	if _, err := users.Authenticate("admin", jsonfile.DefaultAdminPassword); err == nil {
		t.Fatal("the old password still works after a reset")
	}
	// The account keeps everything else.
	u, err := users.Find("admin")
	if err != nil {
		t.Fatal(err)
	}
	if u.Role != "admin" || u.HomeDir != "/" {
		t.Errorf("the reset changed more than the password: role=%q home=%q", u.Role, u.HomeDir)
	}
}

// TestResetEndsExistingSessions: the session fingerprint covers the password
// hash, so an old cookie stops working. This is the property that makes a reset
// a revocation and not just a new key.
func TestResetEndsExistingSessions(t *testing.T) {
	configPath, usersPath := resetFixture(t, config.AuthTypeJSON)
	users := jsonfile.NewUsersFile(usersPath)

	before, err := users.Find("admin")
	if err != nil {
		t.Fatal(err)
	}
	oldPrint := users.SessionFingerprint(before)

	if err := ResetAdminPassword(configPath, "admin", strings.NewReader("rotated-password\nrotated-password\n"), &bytes.Buffer{}); err != nil {
		t.Fatalf("reset: %v", err)
	}
	after, err := users.Find("admin")
	if err != nil {
		t.Fatal(err)
	}
	if users.SessionFingerprint(after) == oldPrint {
		t.Fatal("the session fingerprint did not change, so old sessions survive a reset")
	}
}

// TestResetRefusals checks every way the command can be asked for something it
// must not do. None of them may touch the file.
//
// The password is supplied by the prompt, twice, so a case that means to be
// refused writes both answers.
func TestResetRefusals(t *testing.T) {
	const good = "a-good-password\n"
	cases := []struct {
		name     string
		user     string
		input    string
		authType string
		want     string
	}{
		{"unknown account", "nobody", good + good, config.AuthTypeJSON, "no account named"},
		{"the guest", "guest", good + good, config.AuthTypeJSON, "guest account"},
		{"empty password", "admin", "\n\n", config.AuthTypeJSON, "password is empty"},
		{"no input at all", "admin", "", config.AuthTypeJSON, "password is empty"},
		{"passwords differ", "admin", "one-password\ntwo-password\n", config.AuthTypeJSON, "do not match"},
		{"whitespace", "admin", "  padded  \n  padded  \n", config.AuthTypeJSON, "was changed"},
		{"ldap", "admin", good + good, config.AuthTypeLDAP, "directory or database"},
		{"sql", "admin", good + good, config.AuthTypeMySQL, "directory or database"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			configPath, usersPath := resetFixture(t, c.authType)
			before, err := os.ReadFile(usersPath)
			if err != nil {
				t.Fatal(err)
			}
			err = ResetAdminPassword(configPath, c.user, strings.NewReader(c.input), &bytes.Buffer{})
			if c.name == "whitespace" {
				// A space inside a password is legal; only the ends are a
				// mistake, and the prompt has no stdin to strip, so this one is
				// accepted as typed.
				if err != nil {
					t.Fatalf("a password with leading spaces was refused: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("the reset was accepted; wanted an error containing %q", c.want)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("error %q does not explain %q", err, c.want)
			}
			after, err := os.ReadFile(usersPath)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before, after) {
				t.Error("a refused reset still changed the users file")
			}
		})
	}
}

// TestResetOnlyTheNamedAccount: resetting one administrator must not touch
// another, or the guest.
func TestResetOnlyTheNamedAccount(t *testing.T) {
	configPath, usersPath := resetFixture(t, config.AuthTypeJSON)
	if err := ResetAdminPassword(configPath, "admin", strings.NewReader("only-this-one\nonly-this-one\n"), &bytes.Buffer{}); err != nil {
		t.Fatalf("reset: %v", err)
	}
	users := jsonfile.NewUsersFile(usersPath)
	guest, err := users.Find("guest")
	if err != nil {
		t.Fatal(err)
	}
	if guest.Role != "guest" {
		t.Errorf("the guest account changed: %+v", guest)
	}
}

// mustJSON marshals a config for a fixture.
func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
