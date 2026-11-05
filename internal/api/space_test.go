package api

import (
	"archive/zip"
	"bytes"
	"crypto/rand"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/filegator/filegator/internal/auth"
	"github.com/filegator/filegator/internal/config"
	"github.com/filegator/filegator/internal/storage"
)

// fixedSpace wraps a storage backend with a chosen free/total size, so the
// guard can be tested without filling a real disk.
type fixedSpace struct {
	storage.Storage
	free, total uint64
	err         error
}

func (f fixedSpace) FreeSpace() (uint64, uint64, error) {
	return f.free, f.total, f.err
}

func spaceEnv(t *testing.T) (*env, string) {
	t.Helper()
	e := newEnv(t)
	e.addUser("spaceuser", "pw", "/", auth.RoleUser,
		auth.PermRead, auth.PermWrite, auth.PermUpload,
		auth.PermDownload, auth.PermBatchDownload, auth.PermZip, auth.PermChmod)
	if err := e.srv.Config.Update(func(c *config.Config) error {
		c.Storage.MinFreePercent = 100
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return e, e.login("spaceuser", "pw")
}

func errorCode(t *testing.T, body []byte) string {
	t.Helper()
	var envelope struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("decode error body %q: %v", body, err)
	}
	return envelope.Error.Code
}

func writeZip(t *testing.T, e *env, name string, content string) {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	f, err := zw.Create("inside.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	full := filepath.Join(e.root.Dir(), name)
	if err := os.WriteFile(full, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestContentGrowingWritesAreRefusedAtTheThreshold sets the threshold to 100%,
// which no real disk can satisfy, and checks every guarded operation.
func TestContentGrowingWritesAreRefusedAtTheThreshold(t *testing.T) {
	e, session := spaceEnv(t)
	e.write("editable.md", "old")
	e.write("dest/.keep", "")
	writeZip(t, e, "archive.zip", "unpacked")

	cases := []struct {
		name string
		path string
		body string
	}{
		{"create", "/api/files/create", `{"parent":"/","name":"new.txt"}`},
		{"save", "/api/files/save", `{"path":"/editable.md","content":"new"}`},
		{"copy", "/api/files/copy", `{"paths":["/top.txt"],"target":"/dest"}`},
		{"zip", "/api/files/zip", `{"paths":["/top.txt"],"destination":"/","name":"refused.zip"}`},
		{"unzip", "/api/files/unzip", `{"path":"/archive.zip","destination":"/","folder":"unzipped"}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := e.do(http.MethodPost, c.path, c.body, session)
			if rec.Code != http.StatusInsufficientStorage {
				t.Fatalf("status = %d, want 507; body %s", rec.Code, rec.Body)
			}
			if code := errorCode(t, rec.Body.Bytes()); code != CodeInsufficientStorage {
				t.Errorf("code = %q, want %q", code, CodeInsufficientStorage)
			}
		})
	}

	// Upload is multipart, so it gets the shared helper.
	w := &writeEnv{env: e, session: session}
	rec := w.uploadChunk(map[string]string{
		"resumableFilename":     "blocked.txt",
		"resumableRelativePath": "/",
		"resumableIdentifier":   "blocked",
		"resumableChunkNumber":  "1",
		"resumableTotalChunks":  "1",
		"resumableTotalSize":    "4",
	}, []byte("data"))
	if rec.Code != http.StatusInsufficientStorage {
		t.Fatalf("upload status = %d, want 507; body %s", rec.Code, rec.Body)
	}
}

// TestSmallWritesRemainAvailableAtTheThreshold keeps recovery possible: a
// nearly full disk still allows rename, delete, chmod and mkdir.
func TestSmallWritesRemainAvailableAtTheThreshold(t *testing.T) {
	e, session := spaceEnv(t)
	e.write("file.txt", "x")

	for _, c := range []struct {
		name string
		path string
		body string
	}{
		{"mkdir", "/api/files/mkdir", `{"parent":"/","name":"newdir"}`},
		{"rename", "/api/files/rename", `{"from":"/file.txt","to":"/moved.txt"}`},
		{"chmod", "/api/files/chmod", `{"paths":["/moved.txt"],"mode":"600"}`},
		{"delete", "/api/files/delete", `{"paths":["/moved.txt"]}`},
	} {
		t.Run(c.name, func(t *testing.T) {
			rec := e.do(http.MethodPost, c.path, c.body, session)
			if rec.Code < 200 || rec.Code >= 300 {
				t.Fatalf("status = %d; body %s", rec.Code, rec.Body)
			}
		})
	}
}

func TestMinFreePercentZeroDisablesTheGuard(t *testing.T) {
	e, session := spaceEnv(t)
	if err := e.srv.Config.Update(func(c *config.Config) error {
		c.Storage.MinFreePercent = 0
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	rec := e.do(http.MethodPost, "/api/files/create", `{"parent":"/","name":"allowed.txt"}`, session)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body %s", rec.Code, rec.Body)
	}
}

func TestFreeSpaceProbeFailureAllowsWrites(t *testing.T) {
	e, session := spaceEnv(t)
	e.srv.SetRoot(fixedSpace{
		Storage: e.root,
		free:    0,
		total:   100,
		err:     errors.New("probe failed"),
	}, e.root.Dir())

	rec := e.do(http.MethodPost, "/api/files/create", `{"parent":"/","name":"allowed.txt"}`, session)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body %s", rec.Code, rec.Body)
	}
}

type closeBuffer struct{ bytes.Buffer }

func (c *closeBuffer) Close() error { return nil }

func TestWriteBudgetStopsTheStream(t *testing.T) {
	var dst closeBuffer
	w := (&writeBudget{remaining: 5}).wrap(&dst)
	if _, err := w.Write([]byte("abc")); err != nil {
		t.Fatalf("first write: %v", err)
	}
	if _, err := w.Write([]byte("de")); err != nil {
		t.Fatalf("second write: %v", err)
	}
	if _, err := w.Write([]byte("f")); !errors.Is(err, storage.ErrInsufficientStorage) {
		t.Fatalf("third write = %v, want ErrInsufficientStorage", err)
	}
	if dst.String() != "abcde" {
		t.Errorf("stored %q, want abcde", dst.String())
	}
}

// TestWriteBudgetAtZero checks a zero allowance refuses the first byte, which
// is what an operation exactly at the threshold sees.
func TestWriteBudgetAtZero(t *testing.T) {
	var dst closeBuffer
	w := (&writeBudget{}).wrap(&dst)
	if _, err := w.Write([]byte("x")); !errors.Is(err, storage.ErrInsufficientStorage) {
		t.Fatalf("write = %v, want ErrInsufficientStorage", err)
	}
	if dst.Len() != 0 {
		t.Errorf("wrote %d bytes past the budget", dst.Len())
	}
}

// TestFreeSpaceReservesTheOperationSize covers the case the threshold alone
// missed: free space is above the percentage, but the write would cross it.
func TestFreeSpaceReservesTheOperationSize(t *testing.T) {
	e := newEnv(t)
	if err := e.srv.Config.Update(func(c *config.Config) error {
		c.Storage.MinFreePercent = 10
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	e.srv.SetRoot(fixedSpace{Storage: e.root, free: 150, total: 1000}, e.root.Dir())

	// 10% of 1000 is 100 bytes of reserve; 150 - 100 = 50 bytes of headroom.
	if _, _, err := e.srv.checkFreeSpace(20); err != nil {
		t.Errorf("a 20-byte write should fit: %v", err)
	}
	if _, _, err := e.srv.checkFreeSpace(100); err == nil {
		t.Error("a 100-byte write crosses the reserve but was accepted")
	}
	if _, _, err := e.srv.checkFreeSpace(0); err != nil {
		t.Errorf("free space is above the reserve: %v", err)
	}
}

// TestCopyAndZipReserveTheirSize guards the whole-operation check, not just the
// current percentage: the disk is above the threshold, but the requested work
// would cross it.
func TestCopyAndZipReserveTheirSize(t *testing.T) {
	e := newEnv(t)
	e.addUser("spaceuser", "pw", "/", auth.RoleUser,
		auth.PermRead, auth.PermWrite, auth.PermZip)
	var noise [200]byte
	if _, err := rand.Read(noise[:]); err != nil {
		t.Fatal(err)
	}
	e.write("big.txt", string(noise[:]))
	e.write("small.txt", strings.Repeat("x", 20))
	e.write("dest/.keep", "")
	if err := e.srv.Config.Update(func(c *config.Config) error {
		c.Storage.MinFreePercent = 10
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	e.srv.SetRoot(fixedSpace{Storage: e.root, free: 150, total: 1000}, e.root.Dir())
	session := e.login("spaceuser", "pw")

	rec := e.do(http.MethodPost, "/api/files/copy", `{"paths":["/big.txt"],"target":"/dest"}`, session)
	if rec.Code != http.StatusInsufficientStorage {
		t.Fatalf("large copy status = %d, want 507; body %s", rec.Code, rec.Body)
	}
	rec = e.do(http.MethodPost, "/api/files/copy", `{"paths":["/small.txt"],"target":"/dest"}`, session)
	if rec.Code != http.StatusOK {
		t.Fatalf("small copy status = %d, want 200; body %s", rec.Code, rec.Body)
	}

	rec = e.do(http.MethodPost, "/api/files/zip", `{"paths":["/big.txt"],"destination":"/","name":"big.zip"}`, session)
	if rec.Code != http.StatusInsufficientStorage {
		t.Fatalf("zip status = %d, want 507; body %s", rec.Code, rec.Body)
	}
}

// TestCopyRemovesAPartialTreeWhenTheBudgetRunsOut checks the cleanup: the copy
// starts above the reserve, crosses it part-way through a directory, and the
// destination must not be left behind.
func TestCopyRemovesAPartialTreeWhenTheBudgetRunsOut(t *testing.T) {
	e := newEnv(t)
	e.addUser("spaceuser", "pw", "/", auth.RoleUser,
		auth.PermRead, auth.PermWrite)
	e.write("source/a.bin", strings.Repeat("a", 100))
	e.write("source/b.bin", strings.Repeat("b", 100))
	e.write("dest/.keep", "")
	if err := e.srv.Config.Update(func(c *config.Config) error {
		c.Storage.MinFreePercent = 10
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	// 10% of 1000 is 100; 220 - 100 leaves 120 bytes, enough for the first
	// 100-byte file but not the second.
	e.srv.SetRoot(fixedSpace{Storage: e.root, free: 220, total: 1000}, e.root.Dir())
	session := e.login("spaceuser", "pw")

	rec := e.do(http.MethodPost, "/api/files/copy", `{"paths":["/source"],"target":"/dest"}`, session)
	if rec.Code != http.StatusInsufficientStorage {
		t.Fatalf("status = %d, want 507; body %s", rec.Code, rec.Body)
	}
	if e.root.Exists("/dest/source") {
		t.Error("a partial directory tree was left behind")
	}
}
