package config

import (
	"strings"
	"testing"
)

func TestSSOApplicationNameCannotContainTheSeparator(t *testing.T) {
	c := Default()
	app := SSOApplication{Name: "panel", Secret: strings.Repeat("s", 32), Mode: SSOModeExisting}
	c.SSO.Applications = []SSOApplication{app}
	if err := c.Validate(); err != nil {
		t.Fatalf("valid application rejected: %v", err)
	}

	// "|" separates the fields of a signed handoff.
	app.Name = "panel|eu"
	c.SSO.Applications = []SSOApplication{app}
	err := c.Validate()
	if err == nil {
		t.Fatal("an application name containing the field separator was accepted")
	}
	if !strings.Contains(err.Error(), `"panel|eu"`) {
		t.Errorf("the error does not name the application: %v", err)
	}
}
