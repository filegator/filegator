package api

import (
	"errors"
	"net/http"
	"testing"

	"github.com/filegator/filegator/internal/auth"
	"github.com/filegator/filegator/internal/storage"
)

func TestSplitName(t *testing.T) {
	cases := []struct{ in, base, ext string }{
		{"report.pdf", "report", ".pdf"},
		{"Makefile", "Makefile", ""},
		{".bashrc", ".bashrc", ""},
		{"notes.", "notes.", ""},
		{"archive.tar.gz", "archive", ".tar.gz"},
		{"backup.tar.bz2", "backup", ".tar.bz2"},
		// A name that merely looks compound must keep its own extension, or
		// "my.photos.zip" would become "my (1).photos.zip".
		{"my.photos.zip", "my.photos", ".zip"},
		{"no.extension.here", "no.extension", ".here"},
	}
	for _, c := range cases {
		base, ext := splitName(c.in)
		if base != c.base || ext != c.ext {
			t.Errorf("splitName(%q) = %q, %q; want %q, %q", c.in, base, ext, c.base, c.ext)
		}
	}
}

func TestUpcountName(t *testing.T) {
	cases := map[string]string{
		"report.pdf": "report (1).pdf",
		"Makefile":   "Makefile (1)",
		".bashrc":    ".bashrc (1)",
		// v7 produced "archive.tar (1).gz"; the whole suffix is kept here.
		"archive.tar.gz": "archive (1).tar.gz",
	}
	for in, want := range cases {
		if got := upcountName(in, 1); got != want {
			t.Errorf("upcountName(%q) = %q, want %q", in, got, want)
		}
	}
	if got := upcountName("report.pdf", 3); got != "report (3).pdf" {
		t.Errorf("n=3 gave %q", got)
	}
}

func TestNormalizeCollision(t *testing.T) {
	if got, err := normalizeCollision(""); err != nil || got != CollisionRefuse {
		t.Errorf("empty = %q, %v; want refuse", got, err)
	}
	for _, p := range []string{"refuse", "replace", "upcount", "UPCOUNT", " upcount "} {
		if _, err := normalizeCollision(p); err != nil {
			t.Errorf("%q rejected: %v", p, err)
		}
	}
	if _, err := normalizeCollision("overwrite"); err == nil {
		t.Error("unknown policy accepted")
	}
}

// --- End-to-end through the API ---

func collisionEnv(t *testing.T) *writeEnv {
	t.Helper()
	e := newEnv(t)
	e.addUser("alice2", "pw", "/alice", auth.RoleUser,
		auth.PermRead, auth.PermWrite, auth.PermUpload, auth.PermDownload)
	session := e.login("alice2", "pw")
	return &writeEnv{env: e, session: session}
}

func TestCreateRefusesByDefault(t *testing.T) {
	w := collisionEnv(t)
	w.write("alice/notes.md", "existing")

	rec := w.post("/api/files/create", `{"parent":"/","name":"notes.md"}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status %d, want 409", rec.Code)
	}
	if got := w.storedContent("/notes.md"); got != "existing" {
		t.Errorf("existing file changed: %q", got)
	}
}

func TestCreateUpcount(t *testing.T) {
	w := collisionEnv(t)
	w.write("alice/notes.md", "existing")

	rec := w.post("/api/files/create", `{"parent":"/","name":"notes.md","on_collision":"upcount"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}
	var entry Entry
	decode(t, rec, &entry)
	if entry.Name != "notes (1).md" {
		t.Errorf("name = %q, want notes (1).md", entry.Name)
	}
	// The original must be untouched.
	if got := w.storedContent("/notes.md"); got != "existing" {
		t.Errorf("original changed: %q", got)
	}
}

func TestUpcountTwiceCountsUp(t *testing.T) {
	w := collisionEnv(t)
	w.write("alice/a.txt", "1")
	w.write("alice/a (1).txt", "2")

	rec := w.post("/api/files/create", `{"parent":"/","name":"a.txt","on_collision":"upcount"}`)
	var entry Entry
	decode(t, rec, &entry)
	if entry.Name != "a (2).txt" {
		t.Errorf("name = %q, want a (2).txt", entry.Name)
	}
}

func TestCreateReplace(t *testing.T) {
	w := collisionEnv(t)
	w.write("alice/notes.md", "existing")

	rec := w.post("/api/files/create", `{"parent":"/","name":"notes.md","on_collision":"replace"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}
	var entry Entry
	decode(t, rec, &entry)
	if entry.Name != "notes.md" || entry.Size != 0 {
		t.Errorf("entry = %+v, want a fresh empty notes.md", entry)
	}
}

func TestMkdirRefusesReplace(t *testing.T) {
	w := collisionEnv(t)
	rec := w.post("/api/files/mkdir", `{"parent":"/","name":"docs","on_collision":"replace"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400", rec.Code)
	}
}

func TestRenameUpcount(t *testing.T) {
	w := collisionEnv(t)
	w.write("alice/one.txt", "1")
	w.write("alice/two.txt", "2")

	rec := w.post("/api/files/rename", `{"from":"/two.txt","to":"/one.txt","on_collision":"upcount"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}
	var entry Entry
	decode(t, rec, &entry)
	if entry.Name != "one (1).txt" {
		t.Errorf("name = %q", entry.Name)
	}
	if got := w.storedContent("/one.txt"); got != "1" {
		t.Error("the original was overwritten")
	}
}

func TestCopyUpcount(t *testing.T) {
	w := collisionEnv(t)
	w.write("alice/src.txt", "body")

	rec := w.post("/api/files/copy", `{"paths":["/src.txt"],"target":"/","on_collision":"upcount"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}
	var entries []Entry
	decode(t, rec, &entries)
	if len(entries) != 1 || entries[0].Name != "src (1).txt" {
		t.Fatalf("entries = %+v", entries)
	}
}

func TestMoveUpcount(t *testing.T) {
	w := collisionEnv(t)
	w.write("alice/dest/keep.txt", "keep")
	w.write("alice/keep.txt", "movable")

	rec := w.post("/api/files/move", `{"paths":["/keep.txt"],"target":"/dest","on_collision":"upcount"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}
	if got := w.storedContent("/dest/keep.txt"); got != "keep" {
		t.Error("move overwrote the destination file")
	}
	if got := w.storedContent("/dest/keep (1).txt"); got != "movable" {
		t.Errorf("moved file = %q", got)
	}
}

func TestUnknownCollisionPolicyRejected(t *testing.T) {
	w := collisionEnv(t)
	rec := w.post("/api/files/create", `{"parent":"/","name":"x.txt","on_collision":"overwrite"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400", rec.Code)
	}
}

func TestUploadUpcount(t *testing.T) {
	w := uploadEnv(t)
	w.write("alice/photo.txt", "old")

	rec := w.uploadChunk(map[string]string{
		"resumableFilename":     "photo.txt",
		"resumableRelativePath": "/",
		"resumableIdentifier":   "upcount-id",
		"resumableChunkNumber":  "1",
		"resumableTotalChunks":  "1",
		"resumableTotalSize":    "3",
		"resumableOnCollision":  "upcount",
	}, []byte("new"))
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}
	if got := w.storedContent("/photo.txt"); got != "old" {
		t.Errorf("original changed: %q", got)
	}
	if got := w.storedContent("/photo (1).txt"); got != "new" {
		t.Errorf("uploaded file = %q", got)
	}
}

func TestUploadReplace(t *testing.T) {
	w := uploadEnv(t)
	w.write("alice/photo.txt", "old content")

	rec := w.uploadChunk(map[string]string{
		"resumableFilename":     "photo.txt",
		"resumableRelativePath": "/",
		"resumableIdentifier":   "replace-id",
		"resumableChunkNumber":  "1",
		"resumableTotalChunks":  "1",
		"resumableTotalSize":    "3",
		"resumableOnCollision":  "replace",
	}, []byte("new"))
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}
	if got := w.storedContent("/photo.txt"); got != "new" {
		t.Errorf("content = %q, want the replacement", got)
	}
}

func TestUploadRefusesUnknownPolicy(t *testing.T) {
	w := uploadEnv(t)
	rec := w.uploadChunk(map[string]string{
		"resumableFilename":     "x.txt",
		"resumableRelativePath": "/",
		"resumableIdentifier":   "bad-id",
		"resumableChunkNumber":  "1",
		"resumableTotalChunks":  "1",
		"resumableTotalSize":    "1",
		"resumableOnCollision":  "nonsense",
	}, []byte("x"))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400", rec.Code)
	}
}

func TestCreateNeverOverwritesViaRace(t *testing.T) {
	// Create must fail with ErrExists rather than truncating a file that
	// appeared after the existence check — the excluded-create flag is what
	// makes concurrent uploads safe.
	w := collisionEnv(t)
	fsys, err := w.srv.userFS(mustUser(t, w, "alice2"))
	if err != nil {
		t.Fatal(err)
	}
	w.write("alice/race.txt", "someone else's data")

	if _, err := fsys.Create("/race.txt"); !errors.Is(err, storage.ErrExists) {
		t.Fatalf("Create err = %v, want ErrExists", err)
	}
	if got := w.storedContent("/race.txt"); got != "someone else's data" {
		t.Errorf("content = %q, unchanged expected", got)
	}
}

// mustUser finds a user by name for the race test.
func mustUser(t *testing.T, w *writeEnv, name string) *auth.User {
	t.Helper()
	u, err := w.srv.Users.Find(name)
	if err != nil {
		t.Fatal(err)
	}
	return u
}
