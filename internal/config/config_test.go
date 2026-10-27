package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLoadMissingFileUsesDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	got := s.Get()
	if got.Frontend.AppName != "FileGator" {
		t.Errorf("AppName = %q, want FileGator", got.Frontend.AppName)
	}
	if got.Frontend.Logo != DefaultLogo {
		t.Errorf("Logo = %q, want the embedded %s", got.Frontend.Logo, DefaultLogo)
	}
	// The default must be a file this binary serves, not a URL on the project
	// website: a fresh install has to show its branding offline.
	if got.Frontend.LogoDark != DefaultLogoDark {
		t.Errorf("LogoDark = %q, want the embedded %s", got.Frontend.LogoDark, DefaultLogoDark)
	}
}

func TestLoadPartialFileKeepsDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	data := `{"frontend_config":{"app_name":"Mine"}}`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	got := s.Get()
	if got.Frontend.AppName != "Mine" {
		t.Errorf("AppName = %q, want Mine", got.Frontend.AppName)
	}
	if got.Frontend.UploadChunkSize != 1*1024*1024 {
		t.Errorf("UploadChunkSize = %d, want default", got.Frontend.UploadChunkSize)
	}
}

func TestLoadInvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestUpdatePersistsAndReloads(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	s, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	err = s.Update(func(c *Config) error {
		c.Frontend.AppName = "Changed"
		c.Auth.UsersFile = "/tmp/users.json"
		return nil
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	reloaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Get().Frontend.AppName != "Changed" {
		t.Errorf("AppName after reload = %q", reloaded.Get().Frontend.AppName)
	}
}

func TestUpdateValidationFailureDoesNotChangeState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	s, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	err = s.Update(func(c *Config) error {
		c.Frontend.AppName = ""
		return nil
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	if s.Get().Frontend.AppName == "" {
		t.Error("config was modified despite validation failure")
	}
}

func TestDefaultIsValid(t *testing.T) {
	if err := Default().Validate(); err != nil {
		t.Fatalf("default config invalid: %v", err)
	}
}

func TestSessionDefaults(t *testing.T) {
	c := Default()
	if c.Security.SessionTTL != 8*60*60 {
		t.Errorf("SessionTTL = %d, want 8h", c.Security.SessionTTL)
	}
	if c.Security.SessionKey != "" {
		t.Errorf("SessionKey = %q, want empty until generated", c.Security.SessionKey)
	}
}

func TestSessionKeyValidation(t *testing.T) {
	c := Default()
	c.Security.SessionKey = "deadbeef"
	if err := c.Validate(); err != nil {
		t.Errorf("valid hex key rejected: %v", err)
	}
	c.Security.SessionKey = "not-hex!"
	if err := c.Validate(); err == nil {
		t.Error("invalid hex key accepted")
	}
	c.Security.SessionKey = ""
	c.Security.SessionTTL = 0
	if err := c.Validate(); err == nil {
		t.Error("zero session ttl accepted")
	}
}

func TestIPEntryValidation(t *testing.T) {
	c := Default()
	c.Security.IPAllowlist = []string{"127.0.0.1", "10.0.0.0/8", "2001:db8::/32", ""}
	c.Security.IPDenylist = []string{"192.168.0.0/16"}
	if err := c.Validate(); err != nil {
		t.Errorf("valid entries rejected: %v", err)
	}

	c.Security.IPDenylist = []string{"999.1.1.1"}
	if err := c.Validate(); err == nil {
		t.Error("invalid IP accepted")
	}
	c.Security.IPDenylist = nil
	c.Security.IPAllowlist = []string{"10.0.0.0/33"}
	if err := c.Validate(); err == nil {
		t.Error("invalid CIDR accepted")
	}
}

func TestTrustedIPHeaderDefault(t *testing.T) {
	if Default().Security.TrustedIPHeader != "" {
		t.Error("trusted_ip_header should default to empty (RemoteAddr only)")
	}
}

func TestTrustedIPHeaderValidation(t *testing.T) {
	c := Default()
	for _, h := range []string{"CF-Connecting-IP", "X-Real-IP", "X-Forwarded-For", "X-Custom_Header"} {
		c.Security.TrustedIPHeader = h
		if err := c.Validate(); err != nil {
			t.Errorf("%q rejected: %v", h, err)
		}
	}
	for _, h := range []string{"Bad Header", "Header:Injected", "a\r\nX-Evil", "Ünïcode"} {
		c.Security.TrustedIPHeader = h
		if err := c.Validate(); err == nil {
			t.Errorf("%q accepted", h)
		}
	}
}

func TestSecurityHeadersConfig(t *testing.T) {
	c := Default()
	if c.Security.Headers == nil {
		t.Fatal("Headers should be initialised")
	}
	if len(c.Security.Headers) != 0 {
		t.Errorf("Headers should default to empty overrides, got %v", c.Security.Headers)
	}

	c.Security.Headers = map[string]string{
		"Strict-Transport-Security": "max-age=31536000",
		"X-Frame-Options":           "DENY",
	}
	if err := c.Validate(); err != nil {
		t.Errorf("valid headers rejected: %v", err)
	}

	c.Security.Headers = map[string]string{"Bad Name": "x"}
	if err := c.Validate(); err == nil {
		t.Error("invalid header name accepted")
	}

	c.Security.Headers = map[string]string{"X-Ok": "v\r\nInjected: 1"}
	if err := c.Validate(); err == nil {
		t.Error("CRLF header value accepted")
	}
}

func TestHeadersDeepCopy(t *testing.T) {
	s, err := Load(filepath.Join(t.TempDir(), "c.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Update(func(c *Config) error {
		c.Security.Headers = map[string]string{"X-Test": "orig"}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	got := s.Get()
	got.Security.Headers["X-Test"] = "mutated"
	if s.Get().Security.Headers["X-Test"] != "orig" {
		t.Error("Get did not deep-copy security.headers")
	}
}

func TestLanguageValidation(t *testing.T) {
	c := Default()
	if c.Frontend.Language != "english" {
		t.Errorf("default language = %q", c.Frontend.Language)
	}
	for _, ok := range []string{"english", "german", "portuguese_br"} {
		c.Frontend.Language = ok
		if err := c.Validate(); err != nil {
			t.Errorf("%q rejected: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "English", "../etc", "a b", "fr-FR"} {
		c.Frontend.Language = bad
		if err := c.Validate(); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestLanguageSurvivesSaveLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	s, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Update(func(c *Config) error {
		c.Frontend.Language = "spanish"
		c.Frontend.LanguageSelection = false
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	reloaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	got := reloaded.Get().Frontend
	if got.Language != "spanish" {
		t.Errorf("language = %q", got.Language)
	}
	if got.LanguageSelection {
		t.Error("language_selection should be false")
	}
}

func TestDateFormatDefaultsAndValidation(t *testing.T) {
	c := Default()
	if c.Frontend.DateFormat != "YYYY/MM/DD HH:mm:ss" {
		t.Errorf("default date_format = %q", c.Frontend.DateFormat)
	}
	// v7's own default, which an upgraded config.json may still carry.
	for _, ok := range []string{"", "YYYY/MM/DD HH:mm:ss", "YY/MM/DD hh:mm:ss", "DD.MM.YYYY", "MMMM Do YYYY [at] h:mm A", "dddd, D. MMMM YYYY"} {
		c.Frontend.DateFormat = ok
		if err := c.Validate(); err != nil {
			t.Errorf("%q rejected: %v", ok, err)
		}
	}
	for _, bad := range []string{"YYYY/MM/DD\nHH", "YYYYMMDD\x00", "[YYYY", "YYYY]", strings.Repeat("Y", 65)} {
		c.Frontend.DateFormat = bad
		if err := c.Validate(); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestDateFormatSurvivesSaveLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	s, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Update(func(c *Config) error {
		c.Frontend.DateFormat = "DD.MM.YYYY HH:mm"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	reloaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := reloaded.Get().Frontend.DateFormat; got != "DD.MM.YYYY HH:mm" {
		t.Errorf("date_format = %q", got)
	}
}

// An older config.json has no date_format; the default must fill it in rather
// than leaving the browser to print empty dates.
func TestDateFormatFromOlderConfigKeepsDefault(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"frontend_config":{"app_name":"Mine"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Get().Frontend.DateFormat; got != "YYYY/MM/DD HH:mm:ss" {
		t.Errorf("date_format = %q, want the default", got)
	}
}

func TestGetReturnsCopy(t *testing.T) {
	s, err := Load(filepath.Join(t.TempDir(), "c.json"))
	if err != nil {
		t.Fatal(err)
	}
	got := s.Get()
	got.Frontend.AppName = "Mutated"
	if reflect.DeepEqual(s.Get().Frontend.AppName, "Mutated") {
		t.Error("Get did not return an independent copy")
	}
}

func TestLoggingDefaultsToStdout(t *testing.T) {
	c := Default()
	if !c.Logging.Enabled {
		t.Fatal("logging is off by default")
	}
	if c.Logging.File != "-" {
		t.Errorf("logging.file = %q, want \"-\" (stdout)", c.Logging.File)
	}
	// A supervisor rotates stdout, so the default must not ask for rotation.
	if c.Logging.MaxSize != 0 {
		t.Errorf("logging.max_size = %d, want 0 with stdout", c.Logging.MaxSize)
	}
	// The defaults must be savable as they are, or a fresh install would fail
	// on its first settings save.
	if err := c.Validate(); err != nil {
		t.Errorf("defaults do not validate: %v", err)
	}
}

func TestLoggingRejectsRotationWithStdout(t *testing.T) {
	c := Default()
	c.Logging.MaxSize = 10 * 1024 * 1024
	if err := c.Validate(); err == nil {
		t.Fatal("rotation with stdout was accepted")
	}
}

func TestLoggingFileStillWorks(t *testing.T) {
	c := Default()
	c.Logging.File = "private/logs/app.log"
	c.Logging.MaxSize = 10 * 1024 * 1024
	if err := c.Validate(); err != nil {
		t.Fatalf("file logging rejected: %v", err)
	}
}

func TestLoggingEmptyFileFallsBackToStderr(t *testing.T) {
	c := Default()
	c.Logging.File = ""
	if err := c.Validate(); err != nil {
		t.Fatalf("empty log file rejected: %v", err)
	}
}
