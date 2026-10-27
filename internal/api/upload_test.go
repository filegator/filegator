package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/filegator/filegator/internal/auth"
	"github.com/filegator/filegator/internal/config"
	"github.com/filegator/filegator/internal/security"
)

// uploadEnv returns an env with a user allowed to upload.
func uploadEnv(t *testing.T) *writeEnv {
	t.Helper()
	e := newEnv(t)
	e.addUser("alice", "pw", "/alice", auth.RoleUser,
		auth.PermRead, auth.PermWrite, auth.PermUpload)
	session := e.login("alice", "pw")
	token, _, err := e.srv.CSRF.Token(httptest.NewRequest(http.MethodGet, "/", nil))
	if err != nil {
		t.Fatal(err)
	}
	return &writeEnv{env: e, session: session, csrf: token}
}

// uploadChunk sends one chunk as multipart form data.
func (w *writeEnv) uploadChunk(fields map[string]string, content []byte) *httptest.ResponseRecorder {
	w.t.Helper()
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	for k, v := range fields {
		if err := mw.WriteField(k, v); err != nil {
			w.t.Fatal(err)
		}
	}
	fw, err := mw.CreateFormFile("file", "chunk")
	if err != nil {
		w.t.Fatal(err)
	}
	if _, err := fw.Write(content); err != nil {
		w.t.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		w.t.Fatal(err)
	}

	r := httptest.NewRequest(http.MethodPost, "/api/files/upload", body)
	r.RemoteAddr = "127.0.0.1:1234"
	r.Header.Set("Content-Type", mw.FormDataContentType())
	r.Header.Set(security.CSRFHeader, w.csrf)
	r.AddCookie(&http.Cookie{Name: security.CookieName, Value: w.session})
	r.AddCookie(&http.Cookie{Name: security.CSRFCookieName, Value: w.csrf})
	rec := httptest.NewRecorder()
	w.router.ServeHTTP(rec, r)
	return rec
}

// singleShot uploads content as a one-chunk file.
func (w *writeEnv) singleShot(name, destDir string, content []byte) *httptest.ResponseRecorder {
	w.t.Helper()
	return w.uploadChunk(map[string]string{
		"resumableFilename":     name,
		"resumableRelativePath": destDir,
		"resumableIdentifier":   "id-" + name,
		"resumableChunkNumber":  "1",
		"resumableTotalChunks":  "1",
		"resumableTotalSize":    strconv.Itoa(len(content)),
	}, content)
}

// storedContent reads a file from alice's home.
func (w *writeEnv) storedContent(rel string) string {
	w.t.Helper()
	data, err := os.ReadFile(filepath.Join(w.root.Dir(), "alice", filepath.FromSlash(strings.TrimPrefix(rel, "/"))))
	if err != nil {
		w.t.Fatalf("read %s: %v", rel, err)
	}
	return string(data)
}

// stored returns a file's content from alice's home, or "" when it does not
// exist. It is for assertions that a file was not written, where failing the
// test on a missing file would defeat the point.
func (w *writeEnv) stored(rel string) string {
	w.t.Helper()
	data, err := os.ReadFile(filepath.Join(w.root.Dir(), "alice", filepath.FromSlash(strings.TrimPrefix(rel, "/"))))
	if err != nil {
		return ""
	}
	return string(data)
}

func TestUploadSingleChunk(t *testing.T) {
	w := uploadEnv(t)
	rec := w.singleShot("hello.txt", "/", []byte("hello world"))
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}

	var resp struct {
		Data UploadResponse `json:"data"`
	}
	decode(t, rec, &resp.Data)
	if resp.Data.Status != "stored" {
		t.Errorf("status = %q", resp.Data.Status)
	}
	if resp.Data.Entry == nil || resp.Data.Entry.Name != "hello.txt" {
		t.Errorf("entry = %+v", resp.Data.Entry)
	}
	if got := w.storedContent("/hello.txt"); got != "hello world" {
		t.Errorf("content = %q", got)
	}
}

func TestUploadMultipleChunks(t *testing.T) {
	w := uploadEnv(t)
	parts := []string{"first-", "second-", "third"}
	fields := map[string]string{
		"resumableFilename":     "multi.bin",
		"resumableRelativePath": "/docs",
		"resumableIdentifier":   "multi-id",
		"resumableTotalChunks":  strconv.Itoa(len(parts)),
	}

	if err := os.MkdirAll(filepath.Join(w.root.Dir(), "alice", "docs"), 0o755); err != nil {
		t.Fatal(err)
	}

	for i, part := range parts {
		fields["resumableChunkNumber"] = strconv.Itoa(i + 1)
		rec := w.uploadChunk(fields, []byte(part))
		if rec.Code != http.StatusOK && rec.Code != http.StatusCreated {
			t.Fatalf("chunk %d: status %d body %s", i+1, rec.Code, rec.Body)
		}
		if i < len(parts)-1 {
			if rec.Code != http.StatusOK {
				t.Fatalf("chunk %d: expected 200 while incomplete, got %d", i+1, rec.Code)
			}
			if strings.Contains(rec.Body.String(), `"stored"`) {
				t.Fatalf("chunk %d: stored too early", i+1)
			}
		} else if rec.Code != http.StatusCreated {
			t.Fatalf("final chunk: expected 201, got %d", rec.Code)
		}
	}

	if got := w.storedContent("/docs/multi.bin"); got != "first-second-third" {
		t.Errorf("content = %q", got)
	}
}

func TestUploadOutOfOrderChunks(t *testing.T) {
	w := uploadEnv(t)
	base := map[string]string{
		"resumableFilename":     "ooo.bin",
		"resumableRelativePath": "/",
		"resumableIdentifier":   "ooo-id",
		"resumableTotalChunks":  "3",
	}

	// Send 3, then 1, then 2.
	order := []struct {
		n   string
		got string
	}{{"3", "c"}, {"1", "a"}, {"2", "b"}}
	for _, o := range order {
		base["resumableChunkNumber"] = o.n
		rec := w.uploadChunk(base, []byte(o.got))
		if rec.Code != http.StatusOK && rec.Code != http.StatusCreated {
			t.Fatalf("chunk %s: status %d", o.n, rec.Code)
		}
	}
	if got := w.storedContent("/ooo.bin"); got != "abc" {
		t.Errorf("content = %q, want abc", got)
	}
}

func TestUploadRefusesOverwrite(t *testing.T) {
	w := uploadEnv(t)
	w.write("alice/existing.txt", "original")

	rec := w.singleShot("existing.txt", "/", []byte("replacement"))
	if rec.Code != http.StatusConflict {
		t.Fatalf("status %d, want 409", rec.Code)
	}
	if got := w.storedContent("/existing.txt"); got != "original" {
		t.Errorf("file was overwritten: %q", got)
	}
}

func TestUploadNameTraversalRejected(t *testing.T) {
	w := uploadEnv(t)
	for _, name := range []string{"..", "../evil.txt", "a/b.txt", `a\b.txt`, ""} {
		rec := w.uploadChunk(map[string]string{
			"resumableFilename":     name,
			"resumableRelativePath": "/",
			"resumableIdentifier":   "trav",
			"resumableChunkNumber":  "1",
			"resumableTotalChunks":  "1",
		}, []byte("x"))
		if rec.Code == http.StatusCreated {
			t.Errorf("name %q accepted", name)
		}
	}
	if fileExists(t, filepath.Join(w.root.Dir(), "evil.txt")) {
		t.Fatal("wrote outside the home folder")
	}
}

func TestUploadDestinationTraversalRejected(t *testing.T) {
	w := uploadEnv(t)
	rec := w.singleShot("safe.txt", "/../bob", []byte("x"))
	if rec.Code == http.StatusCreated {
		t.Fatal("upload escaped the home folder")
	}
	if fileExists(t, filepath.Join(w.root.Dir(), "bob", "safe.txt")) {
		t.Fatal("file landed outside the home folder")
	}
}

func TestUploadRequiresPermission(t *testing.T) {
	e := newEnv(t)
	e.addUser("readonly", "pw", "/alice", auth.RoleUser, auth.PermRead)
	session := e.login("readonly", "pw")
	token, _, _ := e.srv.CSRF.Token(httptest.NewRequest(http.MethodGet, "/", nil))

	w := &writeEnv{env: e, session: session, csrf: token}
	rec := w.singleShot("nope.txt", "/", []byte("x"))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d, want 403", rec.Code)
	}
}

func TestUploadRequiresLogin(t *testing.T) {
	e := newEnv(t)
	r := httptest.NewRequest(http.MethodPost, "/api/files/upload", nil)
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, r)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d, want 401", rec.Code)
	}
}

func TestUploadExceedsMaxSize(t *testing.T) {
	w := uploadEnv(t)
	if err := w.srv.Config.Update(func(c *config.Config) error {
		c.Frontend.UploadMaxSize = 10
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	rec := w.singleShot("big.bin", "/", []byte("this is definitely longer than ten bytes"))
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status %d, want 413 (body %s)", rec.Code, rec.Body)
	}
	if w.exists("/big.bin") {
		t.Error("oversized file was stored")
	}
}

func TestUploadInvalidMetadata(t *testing.T) {
	w := uploadEnv(t)
	base := map[string]string{
		"resumableFilename":    "a.txt",
		"resumableIdentifier":  "x",
		"resumableChunkNumber": "1",
		"resumableTotalChunks": "2",
	}
	cases := map[string]map[string]string{
		"missing identifier": {"resumableIdentifier": ""},
		"bad chunk number":   {"resumableChunkNumber": "abc"},
		"zero chunk":         {"resumableChunkNumber": "0"},
		"chunk beyond total": {"resumableChunkNumber": "5"},
		"bad total":          {"resumableTotalChunks": "x"},
		"missing filename":   {"resumableFilename": ""},
	}
	for name, override := range cases {
		fields := map[string]string{}
		for k, v := range base {
			fields[k] = v
		}
		for k, v := range override {
			fields[k] = v
		}
		rec := w.uploadChunk(fields, []byte("x"))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 400", name, rec.Code)
		}
	}
}

func TestUploadStatusReportsProgress(t *testing.T) {
	w := uploadEnv(t)
	fields := map[string]string{
		"resumableFilename":     "status.bin",
		"resumableRelativePath": "/",
		"resumableIdentifier":   "status-id",
		"resumableTotalChunks":  "2",
		"resumableChunkNumber":  "1",
	}
	w.uploadChunk(fields, []byte("part1"))

	rec := w.do(http.MethodGet, "/api/files/upload?resumableFilename=status.bin&resumableIdentifier=status-id&resumableTotalChunks=2&resumableChunkNumber=1", "", w.session)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}
	var resp struct {
		Data UploadResponse `json:"data"`
	}
	decode(t, rec, &resp.Data)
	if resp.Data.Received != 1 || resp.Data.Total != 2 {
		t.Errorf("progress = %+v", resp.Data)
	}
}

func TestUploadStagingIsUserIsolated(t *testing.T) {
	w := uploadEnv(t)
	w.addUser("bob", "pw2", "/bob", auth.RoleUser, auth.PermRead, auth.PermUpload)
	bobSession := w.login("bob", "pw2")

	// Alice stages chunk 1 of a two-chunk upload.
	fields := map[string]string{
		"resumableFilename":     "shared.bin",
		"resumableRelativePath": "/",
		"resumableIdentifier":   "same-id",
		"resumableTotalChunks":  "2",
		"resumableChunkNumber":  "1",
	}
	w.uploadChunk(fields, []byte("alice-part1"))

	// Bob reuses the same identifier. His view of the session must be empty,
	// so his chunk 1 must not complete the upload.
	bob := &writeEnv{env: w.env, session: bobSession, csrf: w.csrf}
	rec := bob.uploadChunk(fields, []byte("bob-part1"))
	if rec.Code != http.StatusOK {
		t.Fatalf("bob chunk 1: status %d (body %s)", rec.Code, rec.Body)
	}
	if strings.Contains(rec.Body.String(), `"stored"`) {
		t.Fatal("bob completed alice's upload")
	}
}

// TestUploadDifferentFilesShareAnIdentifier is the regression test for chunks
// of two different files being spliced into one.
//
// The identifier is not unique across uploads: Resumable.js and this project's
// own client derive it from the file's size and modification time, so two
// people uploading the same file send the same one. Keying the staging folder
// by the identifier alone put both clients' chunks in one folder, and the
// second client's final chunk assembled the first client's data into its file.
func TestUploadDifferentFilesShareAnIdentifier(t *testing.T) {
	w := uploadEnv(t)

	// Client A: chunk 1 of two for "a.txt".
	w.uploadChunk(chunkFields("a.txt", "same-id", 1, 2, 8), []byte("AAAA"))

	// Client B: the final chunk of its own two-chunk upload of "b.txt".
	rec := w.uploadChunk(chunkFields("b.txt", "same-id", 2, 2, 8), []byte("BBBB"))
	// Chunk 1 of b.txt was never sent, so B's upload is incomplete: assembling
	// A's chunk 1 into b.txt is exactly the bug this test exists for.
	if rec.Code == http.StatusCreated || strings.Contains(rec.Body.String(), `"stored"`) {
		t.Fatalf("b.txt was assembled from another upload's chunk: %s", rec.Body)
	}

	// Each upload kept its own chunks in its own folder. B's failed completion
	// removes its own empty folder, so only A's staged upload remains.
	if got := len(w.stagingDirs("same-id")); got != 1 {
		t.Errorf("staging folders = %d, want A's surviving one", got)
	}

	// B retries from the beginning: chunk 1 stages into B's own folder, which
	// is separate from A's, so the two never mix.
	rec = w.uploadChunk(chunkFields("b.txt", "same-id", 1, 2, 8), []byte("BBBB"))
	if rec.Code != http.StatusOK {
		t.Fatalf("b's first chunk: status %d body %s", rec.Code, rec.Body)
	}
	if w.stored("/a.txt") != "" {
		t.Errorf("a.txt was stored before A finished")
	}
	rec = w.uploadChunk(chunkFields("b.txt", "same-id", 2, 2, 8), []byte("BBBB"))
	if rec.Code != http.StatusCreated {
		t.Fatalf("b's last chunk: status %d body %s", rec.Code, rec.Body)
	}
	if got := w.storedContent("/b.txt"); got != "BBBBBBBB" {
		t.Errorf("b.txt = %q, want only b's bytes", got)
	}

	// A's upload is untouched and still assembleable.
	if w.stored("/a.txt") != "" {
		t.Errorf("a.txt was stored before A finished")
	}
	rec = w.uploadChunk(chunkFields("a.txt", "same-id", 2, 2, 8), []byte("AAAA"))
	if rec.Code != http.StatusCreated {
		t.Fatalf("a's last chunk: status %d body %s", rec.Code, rec.Body)
	}
	if got := w.storedContent("/a.txt"); got != "AAAAAAAA" {
		t.Errorf("a.txt = %q, want only a's bytes", got)
	}
}

// TestUploadResumeFindsItsOwnChunks pins the reason the folder is keyed by the
// upload's shape: a resumed upload repeats the same identifier, destination and
// name, so it must find the chunks its earlier session staged.
func TestUploadResumeFindsItsOwnChunks(t *testing.T) {
	w := uploadEnv(t)
	w.uploadChunk(chunkFields("resume.bin", "resume-id", 1, 3, 30), []byte("one"))

	// A new browser session asks how much is already there, exactly as the
	// client does before sending the rest.
	rec := w.getAs("/api/files/upload?resumableFilename=resume.bin&resumableIdentifier=resume-id&resumableTotalChunks=3&resumableChunkNumber=1")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}
	var status UploadResponse
	decode(t, rec, &status)
	if status.Received != 1 {
		t.Fatalf("received = %d, want 1 (a resumed upload must see its chunks)", status.Received)
	}

	for _, chunk := range []struct {
		n    int
		data string
	}{{2, "two"}, {3, "three"}} {
		rec := w.uploadChunk(chunkFields("resume.bin", "resume-id", chunk.n, 3, 30), []byte(chunk.data))
		if rec.Code != http.StatusOK && rec.Code != http.StatusCreated {
			t.Fatalf("chunk %d: status %d body %s", chunk.n, rec.Code, rec.Body)
		}
	}
	if got := w.storedContent("/resume.bin"); got != "onetwothree" {
		t.Errorf("resume.bin = %q", got)
	}
}

// TestCancelRemovesOnlyTheNamedFile checks that cancelling an upload that names
// its file leaves another upload sharing the identifier alone.
func TestCancelRemovesOnlyTheNamedFile(t *testing.T) {
	w := uploadEnv(t)
	w.uploadChunk(chunkFields("first.bin", "shared", 1, 3, 30), []byte("x"))
	w.uploadChunk(chunkFields("second.bin", "shared", 1, 3, 30), []byte("y"))

	body := `{"filename":"first.bin","identifier":"shared"}`
	if rec := w.post("/api/files/upload/cancel", body); rec.Code != http.StatusNoContent {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}
	if got := len(w.stagingDirs("shared")); got != 1 {
		t.Errorf("staging folders left = %d, want the other upload's one", got)
	}
	remaining := w.stagingDirs("shared")
	if len(remaining) == 1 {
		if index, ok := readUploadIndex(remaining[0]); !ok || index.FileName != "second.bin" {
			t.Errorf("cancel removed the wrong upload: %+v", index)
		}
	}
}

func TestUploadCleansStagingAfterSuccess(t *testing.T) {
	w := uploadEnv(t)
	if rec := w.singleShot("clean.txt", "/", []byte("x")); rec.Code != http.StatusCreated {
		t.Fatalf("status %d", rec.Code)
	}

	staging := w.srv.stagingDir()
	remaining := 0
	filepath.WalkDir(staging, func(_ string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			remaining++
		}
		return nil
	})
	if remaining != 0 {
		t.Errorf("%d staged chunks were left behind", remaining)
	}
}

func TestUploadStreamsLargeChunk(t *testing.T) {
	w := uploadEnv(t)
	// 8 MB single chunk, larger than the multipart in-memory buffer.
	content := bytes.Repeat([]byte("0123456789abcdef"), 512*1024)
	rec := w.singleShot("large.bin", "/", content)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}
	full := filepath.Join(w.root.Dir(), "alice", "large.bin")
	info, err := os.Stat(full)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() != int64(len(content)) {
		t.Errorf("size = %d, want %d", info.Size(), len(content))
	}

	f, err := os.Open(full)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	data, _ := io.ReadAll(f)
	if !bytes.Equal(data, content) {
		t.Error("content differs")
	}
}

// cancelUpload discards an upload. The asset's name is left empty so every
// staging folder with that identifier is removed; a cancel that names a file
// removes only that file's folder.
func (w *writeEnv) cancelUpload(identifier string) *httptest.ResponseRecorder {
	w.t.Helper()
	body, err := json.Marshal(CancelRequest{Identifier: identifier})
	if err != nil {
		w.t.Fatal(err)
	}
	return w.post("/api/files/upload/cancel", string(body))
}

// stagingDirs returns the staging folders that hold an upload with identifier,
// matched through the index each folder records.
//
// It deliberately does not recompute the folder's hash: the hash covers the
// upload's destination and file name as well, and a test that reproduced it
// would pass even if the production code stopped writing the index.
func (w *writeEnv) stagingDirs(identifier string) []string {
	w.t.Helper()
	userDir := filepath.Join(w.srv.stagingDir(), safeUserKey("alice"))
	entries, err := os.ReadDir(userDir)
	if err != nil {
		return nil
	}
	var dirs []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(userDir, e.Name())
		if index, ok := readUploadIndex(dir); ok && index.Identifier == identifier {
			dirs = append(dirs, dir)
		}
	}
	return dirs
}

// onlyStagingDir returns the single staging folder for identifier, failing the
// test when there is not exactly one.
func (w *writeEnv) onlyStagingDir(identifier string) string {
	w.t.Helper()
	dirs := w.stagingDirs(identifier)
	if len(dirs) != 1 {
		w.t.Fatalf("staging folders for %q = %d, want 1", identifier, len(dirs))
	}
	return dirs[0]
}

// stagingParts counts staged chunk files for an identifier.
func (w *writeEnv) stagingParts(identifier string) int {
	w.t.Helper()
	count := 0
	for _, dir := range w.stagingDirs(identifier) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".part") {
				count++
			}
		}
	}
	return count
}

// chunkFields builds the Resumable.js fields for one chunk.
func chunkFields(name, identifier string, chunk, total int, size int64) map[string]string {
	return map[string]string{
		"resumableFilename":     name,
		"resumableRelativePath": "/",
		"resumableIdentifier":   identifier,
		"resumableChunkNumber":  strconv.Itoa(chunk),
		"resumableTotalChunks":  strconv.Itoa(total),
		"resumableTotalSize":    strconv.FormatInt(size, 10),
	}
}

func TestCancelDiscardsStagedChunks(t *testing.T) {
	w := uploadEnv(t)
	w.uploadChunk(chunkFields("big.bin", "cancel-me", 1, 3, 300), []byte("first"))

	if got := w.stagingParts("cancel-me"); got != 1 {
		t.Fatalf("staged parts = %d, want 1", got)
	}

	if rec := w.cancelUpload("cancel-me"); rec.Code != http.StatusNoContent {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}
	if got := w.stagingParts("cancel-me"); got != 0 {
		t.Errorf("parts after cancel = %d, want 0", got)
	}
}

func TestCancelOnlyAffectsTheCallersOwnUploads(t *testing.T) {
	w := uploadEnv(t)
	w.uploadChunk(chunkFields("mine.bin", "shared-id", 1, 2, 200), []byte("data"))

	// Another user must not be able to discard that upload by guessing the
	// identifier: the staging folder is keyed by user as well.
	w.addUser("mallory", "pw", "/alice", auth.RoleUser, auth.PermRead, auth.PermUpload)
	session := w.login("mallory", "pw")

	r := httptest.NewRequest(http.MethodPost, "/api/files/upload/cancel",
		strings.NewReader(`{"filename":"x","identifier":"shared-id"}`))
	r.RemoteAddr = "127.0.0.1:1234"
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set(security.CSRFHeader, w.csrf)
	r.AddCookie(&http.Cookie{Name: security.CookieName, Value: session})
	r.AddCookie(&http.Cookie{Name: security.CSRFCookieName, Value: w.csrf})
	rec := httptest.NewRecorder()
	w.router.ServeHTTP(rec, r)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}
	if got := w.stagingParts("shared-id"); got != 1 {
		t.Errorf("another user's cancel discarded the chunks: %d left", got)
	}
}

func TestCancelRequiresLogin(t *testing.T) {
	w := uploadEnv(t)
	r := httptest.NewRequest(http.MethodPost, "/api/files/upload/cancel",
		strings.NewReader(`{"identifier":"x"}`))
	r.RemoteAddr = "127.0.0.1:1234"
	r.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	w.router.ServeHTTP(rec, r)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d, want 401", rec.Code)
	}
}

func TestCancelRejectsMissingIdentifier(t *testing.T) {
	w := uploadEnv(t)
	if rec := w.post("/api/files/upload/cancel", `{}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400", rec.Code)
	}
}

func TestUploadRefusesExistingNameBeforeStoring(t *testing.T) {
	w := uploadEnv(t)
	w.write("alice/taken.txt", "already here")

	rec := w.uploadChunk(chunkFields("taken.txt", "collision-id", 1, 3, 300), []byte("first"))
	if rec.Code != http.StatusConflict {
		t.Fatalf("status %d, want 409", rec.Code)
	}
	// Nothing may be staged for an upload that can never succeed.
	if got := w.stagingParts("collision-id"); got != 0 {
		t.Errorf("staged %d parts for a colliding upload", got)
	}
	if got := w.storedContent("/taken.txt"); got != "already here" {
		t.Errorf("existing file was modified: %q", got)
	}
}

func TestUploadStatusIgnoresStaleChunks(t *testing.T) {
	w := uploadEnv(t)
	// An earlier attempt used a smaller chunk size, leaving five parts.
	for i := 1; i <= 5; i++ {
		w.uploadChunk(chunkFields("restart.bin", "restart-id", i, 5, 500), []byte("x"))
	}

	// The client now reports a larger chunk size, so only two chunks make the
	// file. Reporting 5 of 2 would make the progress bar nonsense.
	rec := w.getAs("/api/files/upload?resumableFilename=restart.bin&resumableIdentifier=restart-id&resumableTotalChunks=2&resumableChunkNumber=1")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	var status UploadResponse
	decode(t, rec, &status)
	if status.Received > status.Total {
		t.Errorf("received %d of %d", status.Received, status.Total)
	}
}

func TestCleanUploadStagingRemovesIdleSessions(t *testing.T) {
	w := uploadEnv(t)
	w.uploadChunk(chunkFields("old.bin", "old-id", 1, 2, 200), []byte("x"))
	w.uploadChunk(chunkFields("new.bin", "new-id", 1, 2, 200), []byte("x"))

	// Age one session past the TTL by moving its mtime back.
	old := w.onlyStagingDir("old-id")
	past := time.Now().Add(-UploadStagingTTL - time.Hour)
	if err := os.Chtimes(old, past, past); err != nil {
		t.Fatal(err)
	}

	w.srv.CleanUploadStaging()

	if got := w.stagingParts("old-id"); got != 0 {
		t.Errorf("idle session survived: %d parts", got)
	}
	if got := w.stagingParts("new-id"); got != 1 {
		t.Errorf("active session was swept: %d parts", got)
	}
}

func TestStartUploadCleanupSweepsImmediatelyAndStops(t *testing.T) {
	w := uploadEnv(t)
	w.uploadChunk(chunkFields("old.bin", "stale-id", 1, 2, 200), []byte("x"))

	old := w.onlyStagingDir("stale-id")
	past := time.Now().Add(-UploadStagingTTL - time.Hour)
	if err := os.Chtimes(old, past, past); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	w.srv.StartUploadCleanup(ctx)

	// The first sweep is synchronous, so no waiting is needed.
	if got := w.stagingParts("stale-id"); got != 0 {
		t.Fatalf("startup sweep did not run: %d parts", got)
	}

	// Cancelling must stop the background goroutine, so a test run does not
	// leak it into the next one.
	cancel()
}
