package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/filegator/filegator/internal/auth"
	"github.com/filegator/filegator/internal/storage"
)

// openCountingStorage reports how many times the handler opened a file. A 304
// must not open one: for S3, Open starts a full GET, which would defeat the
// revalidation saving.
type openCountingStorage struct {
	storage.Storage
	opens int
}

func (s *openCountingStorage) Scope(home string) (storage.FS, error) {
	fsys, err := s.Storage.Scope(home)
	if err != nil {
		return nil, err
	}
	return &openCountingFS{FS: fsys, opens: &s.opens}, nil
}

type openCountingFS struct {
	storage.FS
	opens *int
}

func (f *openCountingFS) Open(p string) (io.ReadCloser, error) {
	(*f.opens)++
	return f.FS.Open(p)
}

func TestETagMatches(t *testing.T) {
	const etag = `W/"abc-1"`
	cases := []struct {
		name   string
		header string
		want   bool
	}{
		{"exact", etag, true},
		{"strong form", `"abc-1"`, true},
		{"list", `"other", ` + etag, true},
		{"wildcard", "*", true},
		{"wildcard in list", `"other", *`, true},
		{"mismatch", `"other"`, false},
		{"empty", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := etagMatches(tc.header, etag); got != tc.want {
				t.Errorf("etagMatches(%q) = %v, want %v", tc.header, got, tc.want)
			}
		})
	}
}

func TestNotModified(t *testing.T) {
	mtime := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	const etag = `W/"abc"`
	cases := []struct {
		name    string
		headers map[string]string
		modtime time.Time
		want    bool
	}{
		{"etag match", map[string]string{"If-None-Match": etag}, mtime, true},
		{"etag mismatch", map[string]string{"If-None-Match": `"other"`}, mtime, false},
		{"date equal", map[string]string{"If-Modified-Since": mtime.Format(http.TimeFormat)}, mtime, true},
		{"date later", map[string]string{"If-Modified-Since": mtime.Add(time.Hour).Format(http.TimeFormat)}, mtime, true},
		{"date earlier", map[string]string{"If-Modified-Since": mtime.Add(-time.Hour).Format(http.TimeFormat)}, mtime, false},
		{"etag wins over date", map[string]string{
			"If-None-Match":     `"other"`,
			"If-Modified-Since": mtime.Format(http.TimeFormat),
		}, mtime, false},
		{"invalid date", map[string]string{"If-Modified-Since": "not a date"}, mtime, false},
		{"zero modtime", map[string]string{"If-Modified-Since": mtime.Format(http.TimeFormat)}, time.Time{}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			for name, value := range tc.headers {
				r.Header.Set(name, value)
			}
			if got := notModified(r, etag, tc.modtime); got != tc.want {
				t.Errorf("notModified = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestInlineETag(t *testing.T) {
	entry := &storage.Entry{Name: "doc.pdf", Size: 10, Modified: time.Unix(1000, 0)}
	const scope = "alice|/alice"
	etag := inlineETag(entry, "application/pdf", scope)

	if again := inlineETag(entry, "application/pdf", scope); again != etag {
		t.Errorf("same entry produced %q and %q", etag, again)
	}
	if !strings.HasPrefix(etag, `W/"`) || !strings.HasSuffix(etag, `"`) {
		t.Errorf("not a weak ETag: %q", etag)
	}
	if got := inlineETag(entry, "text/plain", scope); got == etag {
		t.Error("the content type did not change the ETag")
	}
	if got := inlineETag(entry, "application/pdf", "bob|/bob"); got == etag {
		t.Error("the user scope did not change the ETag")
	}

	backend := *entry
	backend.ETag = `"backend-etag"`
	if got := inlineETag(&backend, "application/pdf", scope); got == etag {
		t.Error("the backend ETag did not change the ETag")
	}

	changed := *entry
	changed.Size = 11
	if got := inlineETag(&changed, "application/pdf", scope); got == etag {
		t.Error("the size did not change the ETag")
	}

	changed = *entry
	changed.Modified = entry.Modified.Add(time.Nanosecond)
	if got := inlineETag(&changed, "application/pdf", scope); got == etag {
		t.Error("the modification time did not change the ETag")
	}
}

func TestInline304DoesNotOpenTheFile(t *testing.T) {
	e := newEnv(t)
	e.addUser("alice", "pw-alice", "/alice", auth.RoleUser, auth.PermRead, auth.PermDownload)
	e.write("alice/doc.pdf", "%PDF-1.4 first")
	session := e.login("alice", "pw-alice")

	counting := &openCountingStorage{Storage: e.srv.ActiveStorage()}
	e.srv.SetRoot(counting, e.root.Dir())

	rec := e.do(http.MethodGet, "/api/files/download?path=/doc.pdf", "", session)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}
	etag := rec.Header().Get("ETag")
	if counting.opens != 1 {
		t.Fatalf("opens = %d after the first response, want 1", counting.opens)
	}

	rec = e.doWithHeaders(http.MethodGet, "/api/files/download?path=/doc.pdf", "", session,
		map[string]string{"If-None-Match": etag})
	if rec.Code != http.StatusNotModified {
		t.Fatalf("status = %d, want 304 (body %s)", rec.Code, rec.Body)
	}
	if counting.opens != 1 {
		t.Errorf("opens = %d after a 304, want 1: the body must not be opened", counting.opens)
	}

	e.write("alice/doc.pdf", "%PDF-1.4 second version")
	rec = e.doWithHeaders(http.MethodGet, "/api/files/download?path=/doc.pdf", "", session,
		map[string]string{"If-None-Match": etag})
	if rec.Code != http.StatusOK {
		t.Fatalf("replaced: status = %d, want 200 (body %s)", rec.Code, rec.Body)
	}
	if counting.opens != 2 {
		t.Errorf("opens = %d after a replacement, want 2", counting.opens)
	}
}
