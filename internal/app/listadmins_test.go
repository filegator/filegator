package app

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// TestListAdmins covers the case that started this: the username is what was
// forgotten, so the command has to say what the usernames are.
func TestListAdmins(t *testing.T) {
	configPath, usersPath := resetFixture(t, "json")

	// A second administrator, the way a hand-edited users.json gets one.
	var recs map[string]map[string]any
	data, err := os.ReadFile(usersPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &recs); err != nil {
		t.Fatal(err)
	}
	recs["3"] = map[string]any{
		"username":    "milena",
		"name":        "Milena",
		"role":        "admin",
		"homedir":     "/team",
		"permissions": "read|write",
		"password":    recs["1"]["password"],
	}
	out, err := json.Marshal(recs)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(usersPath, out, 0o600); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := ListAdmins(configPath, &buf); err != nil {
		t.Fatalf("list: %v", err)
	}
	got := buf.String()

	for _, want := range []string{"admin", "milena", "/team", "STILL THE DEFAULT", "-reset-admin-password"} {
		if !strings.Contains(got, want) {
			t.Errorf("the listing is missing %q:\n%s", want, got)
		}
	}
	// The guest is not an administrator and must not be listed.
	if strings.Contains(got, "guest") {
		t.Errorf("the guest account was listed as an administrator:\n%s", got)
	}
	// No password material of any kind.
	if strings.Contains(got, "$2") || strings.Contains(got, "admin123") {
		t.Errorf("the listing leaked password material:\n%s", got)
	}
}

// TestListAdminsMarksTheDefault checks the one thing an operator needs to see:
// which account is still on the published password.
func TestListAdminsMarksTheDefault(t *testing.T) {
	configPath, _ := resetFixture(t, "json")
	var buf bytes.Buffer
	if err := ListAdmins(configPath, &buf); err != nil {
		t.Fatalf("list: %v", err)
	}
	if !strings.Contains(buf.String(), "STILL THE DEFAULT") {
		t.Fatalf("a fresh install did not report the default password:\n%s", buf.String())
	}
}

// TestListAdminsRefusesWithoutAConfig is the regression test for a real bug:
// with a wrong -config path, config.Load returned the built-in defaults and the
// command listed whatever private/users.json was lying around relative to the
// working directory — someone else's install, reported as if it were the one
// asked for.
func TestListAdminsRefusesWithoutAConfig(t *testing.T) {
	err := ListAdmins("/nonexistent/config.json", &bytes.Buffer{})
	if err == nil {
		t.Fatal("a missing config file was accepted")
	}
	if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("unhelpful error: %v", err)
	}
}

// TestListAdminsRefusesOtherProviders: there is no users file to read.
func TestListAdminsRefusesOtherProviders(t *testing.T) {
	for _, authType := range []string{"ldap", "mysql", "postgres"} {
		configPath, _ := resetFixture(t, authType)
		err := ListAdmins(configPath, &bytes.Buffer{})
		if err == nil || !strings.Contains(err.Error(), "directory or database") {
			t.Errorf("%s: got %v, want a message naming the directory or database", authType, err)
		}
	}
}

// TestListAdminsWithNone says something useful instead of printing an empty
// table.
func TestListAdminsWithNone(t *testing.T) {
	configPath, usersPath := resetFixture(t, "json")
	var recs map[string]map[string]any
	data, _ := os.ReadFile(usersPath)
	json.Unmarshal(data, &recs)
	recs["1"]["role"] = "user"
	out, _ := json.Marshal(recs)
	os.WriteFile(usersPath, out, 0o600)

	var buf bytes.Buffer
	if err := ListAdmins(configPath, &buf); err != nil {
		t.Fatalf("list: %v", err)
	}
	if !strings.Contains(buf.String(), "no administrator account") {
		t.Errorf("unexpected output:\n%s", buf.String())
	}
}

// TestResetNamesTheAccountsItHas covers the other half: a mistyped username on
// the reset must say what the names are, not just that the one given is wrong.
func TestResetNamesTheAccountsItHas(t *testing.T) {
	configPath, _ := resetFixture(t, "json")
	err := ResetAdminPassword(configPath, "adnim", strings.NewReader("password\npassword\n"), &bytes.Buffer{})
	if err == nil {
		t.Fatal("a mistyped username was accepted")
	}
	msg := err.Error()
	for _, want := range []string{"no account named", "the accounts are:", "admin", "-list-admins"} {
		if !strings.Contains(msg, want) {
			t.Errorf("the error is missing %q: %v", want, err)
		}
	}
}
