package config

import (
	"strings"
	"testing"
)

func TestAuthTypeDefaultsToJSON(t *testing.T) {
	c := Default()
	if c.Auth.Type != AuthTypeJSON {
		t.Errorf("type = %q, want json", c.Auth.Type)
	}
}

func TestAuthTypeJSONRequiresUsersFile(t *testing.T) {
	c := Default()
	c.Auth.UsersFile = ""
	if err := c.Validate(); err == nil {
		t.Fatal("missing users_file accepted for json")
	}
}

func TestAuthTypeUnknownRejected(t *testing.T) {
	c := Default()
	c.Auth.Type = "database"
	err := c.Validate()
	if err == nil {
		t.Fatal("unknown auth type accepted")
	}
	if !strings.Contains(err.Error(), "unknown auth.type") {
		t.Errorf("error = %v", err)
	}
}

func TestLDAPValidation(t *testing.T) {
	base := func() *Config {
		c := Default()
		c.Auth.Type = AuthTypeLDAP
		c.Auth.LDAP = LDAP{
			Server:             "ldap://192.168.1.1",
			BaseDN:             "cn=users,dc=example,dc=com",
			UsernameAttribute:  "uid",
			DefaultPermissions: []string{"read", "write"},
		}
		return c
	}

	if err := base().Validate(); err != nil {
		t.Fatalf("valid ldap config rejected: %v", err)
	}

	cases := map[string]func(*Config){
		"missing server":             func(c *Config) { c.Auth.LDAP.Server = "" },
		"missing base_dn":            func(c *Config) { c.Auth.LDAP.BaseDN = "" },
		"missing username attribute": func(c *Config) { c.Auth.LDAP.UsernameAttribute = "" },
		"negative cache ttl":         func(c *Config) { c.Auth.LDAP.UserCacheTTL = -1 },
		"negative timeout":           func(c *Config) { c.Auth.LDAP.Timeout = -1 },
		"unknown permission":         func(c *Config) { c.Auth.LDAP.DefaultPermissions = []string{"teleport"} },
	}
	for name, mutate := range cases {
		c := base()
		mutate(c)
		if err := c.Validate(); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestLDAPDoesNotRequireUsersFile(t *testing.T) {
	c := Default()
	c.Auth.Type = AuthTypeLDAP
	c.Auth.UsersFile = ""
	c.Auth.LDAP = LDAP{
		Server:            "ldap://host",
		BaseDN:            "dc=example,dc=com",
		UsernameAttribute: "uid",
	}
	if err := c.Validate(); err != nil {
		t.Errorf("ldap should not need users_file: %v", err)
	}
}
