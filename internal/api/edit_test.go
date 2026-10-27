package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/filegator/filegator/internal/auth"
	"github.com/filegator/filegator/internal/config"
)

func TestReadContent(t *testing.T) {
	w := newWriteEnv(t)
	e := w.env
	session := e.login("alice", "pw-alice")

	rec := e.do(http.MethodGet, "/api/files/content?path=/notes.md", "", session)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}
	var result struct {
		Data EditContent `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Data.Content != "notes" {
		t.Errorf("content = %q", result.Data.Content)
	}
	if result.Data.Name != "notes.md" || result.Data.Path != "/notes.md" {
		t.Errorf("entry = %+v", result.Data)
	}
	if result.Data.Modified == "" {
		t.Error("modified missing")
	}
}

func TestReadContentRejectsNonEditable(t *testing.T) {
	w := newWriteEnv(t)
	e := w.env
	session := e.login("alice", "pw-alice")

	// report.txt is editable by default; a .pdf is not in the list.
	if err := os.WriteFile(filepath.Join(e.root.Dir(), "alice", "doc.pdf"), []byte("%PDF"), 0o644); err != nil {
		t.Fatal(err)
	}
	rec := e.do(http.MethodGet, "/api/files/content?path=/doc.pdf", "", session)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400 (.pdf is not in the editable list)", rec.Code)
	}
}

func TestReadContentRejectsBinary(t *testing.T) {
	w := newWriteEnv(t)
	e := w.env
	session := e.login("alice", "pw-alice")

	// A .json file that actually contains NUL bytes.
	full := filepath.Join(e.root.Dir(), "alice", "binary.json")
	if err := os.WriteFile(full, []byte{'{', 0x00, '}'}, 0o644); err != nil {
		t.Fatal(err)
	}

	rec := e.do(http.MethodGet, "/api/files/content?path=/binary.json", "", session)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "not text") {
		t.Errorf("body = %s", rec.Body)
	}
}

func TestReadContentTooLarge(t *testing.T) {
	w := newWriteEnv(t)
	e := w.env
	session := e.login("alice", "pw-alice")

	big := strings.Repeat("a", maxEditSize+10)
	if err := os.WriteFile(filepath.Join(e.root.Dir(), "alice", "big.md"), []byte(big), 0o644); err != nil {
		t.Fatal(err)
	}

	rec := e.do(http.MethodGet, "/api/files/content?path=/big.md", "", session)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status %d, want 413", rec.Code)
	}
}

func TestReadContentDirectory(t *testing.T) {
	w := newWriteEnv(t)
	e := w.env
	session := e.login("alice", "pw-alice")

	rec := e.do(http.MethodGet, "/api/files/content?path=/docs", "", session)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d, want 400", rec.Code)
	}
}

func TestReadContentRequiresPermission(t *testing.T) {
	w := newWriteEnv(t)
	e := w.env
	e.srv.Users.Delete("alice")
	e.addUser("alice", "pw", "/alice", auth.RoleUser)
	session := e.login("alice", "pw")

	rec := e.do(http.MethodGet, "/api/files/content?path=/notes.md", "", session)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d, want 403", rec.Code)
	}
}

func TestReadContentCannotEscapeHome(t *testing.T) {
	w := newWriteEnv(t)
	e := w.env
	session := e.login("alice", "pw-alice")

	for _, p := range []string{"/../secret.txt", "/../../etc/passwd"} {
		rec := e.do(http.MethodGet, "/api/files/content?path="+p, "", session)
		if rec.Code == http.StatusOK {
			t.Errorf("path %q was readable", p)
		}
	}
}

func TestReadContentRespectsEditableConfig(t *testing.T) {
	w := newWriteEnv(t)
	e := w.env
	session := e.login("alice", "pw-alice")

	if err := os.WriteFile(filepath.Join(e.root.Dir(), "alice", "doc.pdf"), []byte("%PDF"), 0o644); err != nil {
		t.Fatal(err)
	}

	// .pdf is not editable by default.
	rec := e.do(http.MethodGet, "/api/files/content?path=/doc.pdf", "", session)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d", rec.Code)
	}

	// Adding it to the editable list makes it readable.
	if err := e.srv.Config.Update(func(c *config.Config) error {
		c.Frontend.Editable = append(c.Frontend.Editable, ".pdf")
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	rec = e.do(http.MethodGet, "/api/files/content?path=/doc.pdf", "", session)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d after enabling .pdf (body %s)", rec.Code, rec.Body)
	}
}

func TestIsBinary(t *testing.T) {
	if isBinary([]byte("plain text")) {
		t.Error("text detected as binary")
	}
	if !isBinary([]byte{'a', 0x00, 'b'}) {
		t.Error("NUL byte not detected")
	}
	if !isBinary([]byte("hello world\n\x00binary")) {
		t.Error("binary after text not detected")
	}
}
