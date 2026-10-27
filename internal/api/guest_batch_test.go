package api

import (
	"archive/zip"
	"bytes"
	"io"
	"net/http"
	"testing"

	"github.com/filegator/filegator/internal/auth"
)

func TestGuestCanBatchDownload(t *testing.T) {
	e := guestEnv(t, auth.PermRead, auth.PermDownload, auth.PermBatchDownload)

	rec := e.anon(http.MethodGet, "/api/files/batch?path=/notes.md", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}

	data := rec.Body.Bytes()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("not a zip: %v", err)
	}
	if len(zr.File) != 1 {
		t.Fatalf("entries = %d", len(zr.File))
	}
	rc, err := zr.Open(zr.File[0].Name)
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	content, _ := io.ReadAll(rc)
	if string(content) != "notes" {
		t.Errorf("content = %q", content)
	}
}

func TestGuestBatchDownloadStillNeedsThePermission(t *testing.T) {
	e := guestEnv(t, auth.PermRead, auth.PermDownload)

	rec := e.anon(http.MethodGet, "/api/files/batch?path=/notes.md", "")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d, want 403 without batchdownload", rec.Code)
	}
}
