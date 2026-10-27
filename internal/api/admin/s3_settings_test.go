package admin

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/filegator/filegator/internal/config"
)

func TestStorageConfigS3Fields(t *testing.T) {
	w := adminEnv(t)
	if err := w.srv.Config.Update(func(c *config.Config) error {
		c.Storage.S3 = config.S3{
			Bucket:       "mybucket",
			Region:       "eu-west-1",
			Endpoint:     "https://nyc3.digitaloceanspaces.com",
			AccessKey:    "AKIA",
			SecretKey:    "very-secret",
			UsePathStyle: true,
			Prefix:       "repo",
			Timeout:      30,
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	rec := w.req(http.MethodGet, "/api/admin/storage", "")
	body := rec.Body.String()
	if containsStr(body, "very-secret") {
		t.Errorf("secret key leaked: %s", body)
	}

	var result struct {
		Data StorageConfig `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	got := result.Data
	if got.S3Bucket != "mybucket" || got.S3Region != "eu-west-1" {
		t.Errorf("s3 fields = %+v", got)
	}
	if !got.S3HasSecret {
		t.Error("s3_has_secret should be true")
	}
	if !got.S3UsePathStyle || got.S3Prefix != "repo" {
		t.Errorf("path style or prefix lost: %+v", got)
	}
}

func TestSetStorageConfigRejectsBadS3(t *testing.T) {
	w := adminEnv(t)
	before := w.srv.StorageDescription()

	rec := w.submit("/api/admin/storage", w.storageBody(func(c *StorageConfig) {
		c.Type = config.StorageTypeS3
		c.S3Bucket = ""
		c.S3AccessKey = ""
		c.S3SecretKey = ""
	}))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400 (body %s)", rec.Code, rec.Body)
	}
	if after := w.srv.StorageDescription(); after != before {
		t.Error("storage changed after a failed switch")
	}
}

func TestSetStorageConfigKeepsS3SecretWhenBlank(t *testing.T) {
	w := adminEnv(t)
	if err := w.srv.Config.Update(func(c *config.Config) error {
		c.Storage.S3 = config.S3{Bucket: "b", Region: "us-east-1", AccessKey: "a", SecretKey: "stored-secret"}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	rec := w.submit("/api/admin/storage", w.storageBody(func(c *StorageConfig) {
		c.Type = config.StorageTypeS3
		c.S3Bucket = "b"
		c.S3Region = "us-east-1"
		c.S3AccessKey = "a"
		c.S3SecretKey = ""
		c.S3HasSecret = true
		c.S3Endpoint = "http://127.0.0.1:1"
		c.S3Timeout = 1
	}))
	// Must fail on the connection, not on missing credentials.
	if containsStr(rec.Body.String(), "access_key and secret_key") {
		t.Errorf("stored secret was not reused: %s", rec.Body)
	}
}
