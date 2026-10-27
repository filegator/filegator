package config

import (
	"strings"
	"testing"
)

func TestStorageTypeDefaultsToLocal(t *testing.T) {
	c := Default()
	if c.Storage.Type != StorageTypeLocal {
		t.Errorf("type = %q, want local", c.Storage.Type)
	}
	if c.Storage.Root != "repository" {
		t.Errorf("root = %q", c.Storage.Root)
	}
}

func TestStorageTypeLocalAccepted(t *testing.T) {
	c := Default()
	c.Storage.Type = StorageTypeLocal
	if err := c.Validate(); err != nil {
		t.Errorf("local rejected: %v", err)
	}
	// An empty type means local too, for configs written before it existed.
	c.Storage.Type = ""
	if err := c.Validate(); err != nil {
		t.Errorf("empty type rejected: %v", err)
	}
}

func TestStorageTypeUnknownRejected(t *testing.T) {
	c := Default()
	c.Storage.Type = "ftp"
	err := c.Validate()
	if err == nil {
		t.Fatal("unknown type accepted")
	}
	if !strings.Contains(err.Error(), "unknown storage.type") {
		t.Errorf("error = %v", err)
	}
}

func TestSFTPValidation(t *testing.T) {
	base := func() *Config {
		c := Default()
		c.Storage.Type = StorageTypeSFTP
		c.Storage.SFTP = SFTP{
			Host:     "example.com",
			User:     "demo",
			Password: "secret",
		}
		return c
	}

	if err := base().Validate(); err != nil {
		t.Fatalf("valid sftp config rejected: %v", err)
	}

	cases := map[string]func(*Config){
		"missing host":      func(c *Config) { c.Storage.SFTP.Host = "" },
		"missing user":      func(c *Config) { c.Storage.SFTP.User = "" },
		"no credentials":    func(c *Config) { c.Storage.SFTP.Password = "" },
		"port out of range": func(c *Config) { c.Storage.SFTP.Port = 70000 },
	}
	for name, mutate := range cases {
		c := base()
		mutate(c)
		if err := c.Validate(); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestSFTPKeyOnlyIsAccepted(t *testing.T) {
	c := Default()
	c.Storage.Type = StorageTypeSFTP
	c.Storage.SFTP = SFTP{
		Host:       "example.com",
		User:       "demo",
		PrivateKey: "/home/demo/.ssh/id_ed25519",
	}
	if err := c.Validate(); err != nil {
		t.Errorf("key-only config rejected: %v", err)
	}
}

func TestSFTPDefaultPortAllowed(t *testing.T) {
	c := Default()
	c.Storage.Type = StorageTypeSFTP
	c.Storage.SFTP = SFTP{Host: "h", User: "u", Password: "p"}
	// Port 0 means "use 22".
	if err := c.Validate(); err != nil {
		t.Errorf("port 0 rejected: %v", err)
	}
}

func TestS3Validation(t *testing.T) {
	base := func() *Config {
		c := Default()
		c.Storage.Type = StorageTypeS3
		c.Storage.Root = "" // S3 has no root path
		c.Storage.S3 = S3{
			Bucket:    "b",
			Region:    "us-east-1",
			AccessKey: "k",
			SecretKey: "s",
		}
		return c
	}

	if err := base().Validate(); err != nil {
		t.Fatalf("valid s3 config rejected: %v", err)
	}

	cases := map[string]func(*Config){
		"missing bucket":   func(c *Config) { c.Storage.S3.Bucket = "" },
		"missing creds":    func(c *Config) { c.Storage.S3.AccessKey = "" },
		"missing secret":   func(c *Config) { c.Storage.S3.SecretKey = "" },
		"missing region":   func(c *Config) { c.Storage.S3.Region = "" },
		"negative timeout": func(c *Config) { c.Storage.S3.Timeout = -1 },
	}
	for name, mutate := range cases {
		c := base()
		mutate(c)
		if err := c.Validate(); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestS3DoesNotRequireRoot(t *testing.T) {
	c := Default()
	c.Storage.Type = StorageTypeS3
	c.Storage.Root = ""
	c.Storage.S3 = S3{Bucket: "b", Region: "r", AccessKey: "k", SecretKey: "s"}
	if err := c.Validate(); err != nil {
		t.Errorf("s3 should not need storage.root: %v", err)
	}

	// But local still does.
	c2 := Default()
	c2.Storage.Root = ""
	if err := c2.Validate(); err == nil {
		t.Error("local storage accepted an empty root")
	}
}
