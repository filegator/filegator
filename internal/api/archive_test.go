package api

import (
	"archive/zip"
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/filegator/filegator/internal/auth"
	"github.com/filegator/filegator/internal/security"
)

// archiveEnv returns an environment with a user that may read, write and zip.
func archiveEnv(t *testing.T) *writeEnv {
	t.Helper()
	e := newEnv(t)
	e.addUser("zipper", "pw", "/alice", auth.RoleUser,
		auth.PermRead, auth.PermWrite, auth.PermZip, auth.PermDownload, auth.PermBatchDownload)
	session := e.login("zipper", "pw")
	return &writeEnv{env: e, session: session}
}

// archivePOST issues a POST as the session user. newEnv leaves CSRF off, so
// the token is not needed, but the header is sent anyway for realism.
func (w *writeEnv) archivePOST(path, body string) *httptest.ResponseRecorder {
	w.t.Helper()
	r := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	r.RemoteAddr = "127.0.0.1:1234"
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set(security.CSRFHeader, w.csrf)
	r.AddCookie(&http.Cookie{Name: security.CookieName, Value: w.session})
	r.AddCookie(&http.Cookie{Name: security.CSRFCookieName, Value: w.csrf})
	rec := httptest.NewRecorder()
	w.router.ServeHTTP(rec, r)
	return rec
}

// getAs issues a GET as the session user.
func (w *writeEnv) getAs(path string) *httptest.ResponseRecorder {
	w.t.Helper()
	r := httptest.NewRequest(http.MethodGet, path, nil)
	r.RemoteAddr = "127.0.0.1:1234"
	r.AddCookie(&http.Cookie{Name: security.CookieName, Value: w.session})
	rec := httptest.NewRecorder()
	w.router.ServeHTTP(rec, r)
	return rec
}

func TestZipCreatesArchive(t *testing.T) {
	w := archiveEnv(t)
	rec := w.archivePOST("/api/files/zip", `{"paths":["/notes.md"],"destination":"/","name":"out.zip"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}

	var entry Entry
	decode(t, rec, &entry)
	if entry.Name != "out.zip" || entry.Type != "file" {
		t.Errorf("entry = %+v", entry)
	}
	if !w.exists("/out.zip") {
		t.Error("archive not created on disk")
	}

	// The archive contains the file.
	data, err := os.ReadFile(filepath.Join(w.root.Dir(), "alice", "out.zip"))
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("not a valid zip: %v", err)
	}
	if len(zr.File) != 1 || zr.File[0].Name != "notes.md" {
		t.Errorf("entries = %v", zr.File[0].Name)
	}
}

func TestZipDefaultName(t *testing.T) {
	w := archiveEnv(t)
	rec := w.archivePOST("/api/files/zip", `{"paths":["/notes.md"]}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}
	var entry Entry
	decode(t, rec, &entry)
	if entry.Name != "archive.zip" {
		t.Errorf("name = %q, want the configured default", entry.Name)
	}
}

func TestZipDirectory(t *testing.T) {
	w := archiveEnv(t)
	rec := w.archivePOST("/api/files/zip", `{"paths":["/docs"],"name":"docs.zip"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}
}

func TestZipRequiresPermission(t *testing.T) {
	e := newEnv(t)
	e.addUser("reader", "pw", "/alice", auth.RoleUser, auth.PermRead)
	session := e.login("reader", "pw")

	rec := e.do(http.MethodPost, "/api/files/zip", `{"paths":["/notes.md"]}`, session)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d, want 403", rec.Code)
	}
}

func TestZipRefusesOverwrite(t *testing.T) {
	w := archiveEnv(t)
	w.write("alice/taken.zip", "existing")
	rec := w.archivePOST("/api/files/zip", `{"paths":["/notes.md"],"name":"taken.zip"}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status %d, want 409", rec.Code)
	}
}

func TestZipRejectsNoPaths(t *testing.T) {
	w := archiveEnv(t)
	rec := w.archivePOST("/api/files/zip", `{"paths":[]}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400", rec.Code)
	}
}

func TestUnzipExtractsIntoSubfolder(t *testing.T) {
	w := archiveEnv(t)

	// Build an archive through the API, then extract it.
	if rec := w.archivePOST("/api/files/zip", `{"paths":["/notes.md"],"name":"pack.zip"}`); rec.Code != http.StatusCreated {
		t.Fatalf("zip: status %d body %s", rec.Code, rec.Body)
	}

	rec := w.archivePOST("/api/files/unzip", `{"path":"/pack.zip"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("unzip: status %d body %s", rec.Code, rec.Body)
	}

	var result struct {
		Folder  string  `json:"folder"`
		Entries []Entry `json:"entries"`
	}
	decode(t, rec, &result)

	if result.Folder != "/pack" {
		t.Errorf("folder = %q, want /pack (from the archive name)", result.Folder)
	}
	if len(result.Entries) != 1 || result.Entries[0].Name != "notes.md" {
		t.Errorf("entries = %+v", result.Entries)
	}
}

func TestUnzipCustomFolder(t *testing.T) {
	w := archiveEnv(t)
	w.archivePOST("/api/files/zip", `{"paths":["/notes.md"],"name":"pack.zip"}`)

	rec := w.archivePOST("/api/files/unzip", `{"path":"/pack.zip","folder":"custom"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}
	if !w.exists("/custom/notes.md") {
		t.Error("file not extracted into the custom folder")
	}
}

func TestUnzipRefusesExistingFolder(t *testing.T) {
	w := archiveEnv(t)
	w.archivePOST("/api/files/zip", `{"paths":["/notes.md"],"name":"pack.zip"}`)
	if err := os.MkdirAll(filepath.Join(w.root.Dir(), "alice", "pack"), 0o755); err != nil {
		t.Fatal(err)
	}

	rec := w.archivePOST("/api/files/unzip", `{"path":"/pack.zip"}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status %d, want 409", rec.Code)
	}
}

func TestUnzipRejectsNonArchive(t *testing.T) {
	w := archiveEnv(t)
	w.write("alice/broken.zip", "not a zip")
	rec := w.archivePOST("/api/files/unzip", `{"path":"/broken.zip"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400", rec.Code)
	}
}

func TestBatchStreamsZip(t *testing.T) {
	w := archiveEnv(t)
	rec := w.getAs("/api/files/batch?path=/notes.md&path=/docs/report.txt")

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/zip" {
		t.Errorf("content type = %q", ct)
	}
	if cd := rec.Header().Get("Content-Disposition"); !strings.Contains(cd, "attachment") {
		t.Errorf("content disposition = %q", cd)
	}

	data := rec.Body.Bytes()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("not a zip: %v", err)
	}
	if len(zr.File) != 2 {
		t.Fatalf("entries = %d, want 2", len(zr.File))
	}

	// Content must survive the round trip.
	rc, err := zr.Open(zr.File[0].Name)
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	content, _ := io.ReadAll(rc)
	if len(content) == 0 {
		t.Error("extracted content is empty")
	}
}

func TestBatchRequiresBatchPermission(t *testing.T) {
	e := newEnv(t)
	e.addUser("reader", "pw", "/alice", auth.RoleUser,
		auth.PermRead, auth.PermDownload) // no batchdownload
	session := e.login("reader", "pw")

	rec := e.do(http.MethodGet, "/api/files/batch?path=/notes.md", "", session)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d, want 403", rec.Code)
	}
}

func TestBatchRequiresPaths(t *testing.T) {
	w := archiveEnv(t)
	rec := w.getAs("/api/files/batch")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400", rec.Code)
	}
}

// getDownload fetches the download endpoint as the session user.
func (w *writeEnv) getDownload(path string) *httptest.ResponseRecorder {
	w.t.Helper()
	return w.getAs("/api/files/download?path=" + url.QueryEscape(path))
}

func TestDownloadServesImagesInline(t *testing.T) {
	w := archiveEnv(t)
	w.write("alice/photo.png", "\x89PNG\r\n\x1a\n")

	rec := w.getDownload("/photo.png")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}
	if got := rec.Header().Get("Content-Disposition"); !strings.HasPrefix(got, "inline") {
		t.Errorf("disposition = %q, want inline so the gallery can show it", got)
	}
	if got := rec.Header().Get("Content-Type"); got != "image/png" {
		t.Errorf("content type = %q", got)
	}
}

func TestDownloadKeepsOtherFilesAsAttachments(t *testing.T) {
	w := archiveEnv(t)

	// An uploaded HTML file must never render on this origin: with
	// Content-Disposition: attachment the browser downloads it instead, and
	// even an inline render would not execute under nosniff.
	w.write("alice/evil.html", "<script>alert(1)</script>")
	rec := w.getDownload("/evil.html")
	if got := rec.Header().Get("Content-Disposition"); !strings.HasPrefix(got, "attachment") {
		t.Errorf("disposition = %q, want attachment", got)
	}

	// SVG is a picture, but it can carry script, so it stays a download.
	w.write("alice/logo.svg", "<svg/>")
	rec = w.getDownload("/logo.svg")
	if got := rec.Header().Get("Content-Disposition"); !strings.HasPrefix(got, "attachment") {
		t.Errorf("svg disposition = %q, want attachment", got)
	}
	if got := rec.Header().Get("Content-Type"); got == "image/svg+xml" {
		t.Errorf("svg sent as %q, which the browser would execute", got)
	}
}

func TestDownloadServesPDFInline(t *testing.T) {
	w := archiveEnv(t)
	w.write("alice/doc.pdf", "%PDF-1.4")

	rec := w.getDownload("/doc.pdf")
	if got := rec.Header().Get("Content-Disposition"); !strings.HasPrefix(got, "inline") {
		t.Errorf("disposition = %q, want inline", got)
	}
}
