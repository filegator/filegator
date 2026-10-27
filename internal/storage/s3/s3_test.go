package s3

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/filegator/filegator/internal/config"
	"github.com/filegator/filegator/internal/storage"
)

// fakeS3 is a minimal in-process S3 server: it stores objects in a map and
// implements just enough of the REST API (PUT, GET, HEAD, DELETE,
// ListObjectsV2) to exercise the client over real HTTP.
type fakeS3 struct {
	mu      sync.RWMutex
	objects map[string][]byte
	// requests records what the client sent, so tests can assert on headers.
	requests []*http.Request
	// failWith makes the next request return this status.
	failWith int
}

func newFakeS3(t *testing.T) (*fakeS3, *httptest.Server) {
	t.Helper()
	f := &fakeS3{objects: map[string][]byte{}}
	srv := httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(srv.Close)
	return f, srv
}

// keyFromPath extracts the object key from a path-style URL.
func (f *fakeS3) keyFromPath(p string) string {
	// Strip the leading "/bucket".
	trimmed := strings.TrimPrefix(p, "/")
	if i := strings.Index(trimmed, "/"); i >= 0 {
		return trimmed[i+1:]
	}
	return ""
}

func (f *fakeS3) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	f.requests = append(f.requests, r.Clone(r.Context()))
	fail := f.failWith
	f.failWith = 0
	f.mu.Unlock()

	if fail != 0 {
		w.WriteHeader(fail)
		fmt.Fprint(w, `<Error><Code>InternalError</Code><Message>injected</Message></Error>`)
		return
	}

	key := f.keyFromPath(r.URL.Path)

	f.mu.Lock()
	defer f.mu.Unlock()

	switch r.Method {
	case http.MethodGet:
		if r.URL.Query().Get("list-type") == "2" {
			f.list(w, r)
			return
		}
		data, ok := f.objects[key]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, `<Error><Code>NoSuchKey</Code><Message>missing</Message></Error>`)
			return
		}
		w.Header().Set("Content-Length", fmt.Sprint(len(data)))
		w.Write(data)

	case http.MethodHead:
		data, ok := f.objects[key]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Length", fmt.Sprint(len(data)))
		w.Header().Set("Last-Modified", time.Now().UTC().Format(http.TimeFormat))

	case http.MethodPut:
		body, _ := io.ReadAll(r.Body)
		if src := r.Header.Get("X-Amz-Copy-Source"); src != "" {
			// CopyObject: "bucket/key" possibly URL-escaped.
			srcKey := strings.TrimPrefix(src, "/")
			if i := strings.Index(srcKey, "/"); i >= 0 {
				srcKey = srcKey[i+1:]
			}
			data, ok := f.objects[srcKey]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			f.objects[key] = data
			fmt.Fprint(w, `<CopyObjectResult><ETag>"x"</ETag></CopyObjectResult>`)
			return
		}
		f.objects[key] = body
		w.WriteHeader(http.StatusOK)

	case http.MethodDelete:
		delete(f.objects, key)

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// list implements ListObjectsV2 with delimiter support.
func (f *fakeS3) list(w http.ResponseWriter, r *http.Request) {
	prefix := r.URL.Query().Get("prefix")
	delimiter := r.URL.Query().Get("delimiter")

	keys := make([]string, 0, len(f.objects))
	for k := range f.objects {
		if strings.HasPrefix(k, prefix) {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)

	seen := map[string]bool{}
	var contents, prefixes []string
	for _, k := range keys {
		rest := strings.TrimPrefix(k, prefix)
		if delimiter != "" {
			if i := strings.Index(rest, delimiter); i >= 0 {
				p := prefix + rest[:i+1]
				if !seen[p] {
					seen[p] = true
					prefixes = append(prefixes, p)
				}
				continue
			}
		}
		contents = append(contents, k)
	}

	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?><ListBucketResult>`)
	b.WriteString("<IsTruncated>false</IsTruncated>")
	for _, k := range contents {
		fmt.Fprintf(&b, "<Contents><Key>%s</Key><Size>%d</Size><LastModified>%s</LastModified></Contents>",
			k, len(f.objects[k]), time.Now().UTC().Format(time.RFC3339))
	}
	for _, p := range prefixes {
		fmt.Fprintf(&b, "<CommonPrefixes><Prefix>%s</Prefix></CommonPrefixes>", p)
	}
	b.WriteString("</ListBucketResult>")
	w.Header().Set("Content-Type", "application/xml")
	io.WriteString(w, b.String())
}

// newTestBackend starts a fake server and connects the backend to it.
func newTestBackend(t *testing.T) (*Backend, *fakeS3) {
	t.Helper()
	f, srv := newFakeS3(t)
	b, err := New(config.S3{
		Bucket:       "testbucket",
		Region:       "us-east-1",
		Endpoint:     srv.URL,
		AccessKey:    "AKID",
		SecretKey:    "SECRET",
		UsePathStyle: true,
		Timeout:      5,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return b, f
}

func TestS3PutGet(t *testing.T) {
	b, _ := newTestBackend(t)
	fsys, err := b.Scope("/")
	if err != nil {
		t.Fatalf("Scope: %v", err)
	}

	w, err := fsys.Create("/hello.txt")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := w.Write([]byte("hello s3")); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	r, err := fsys.Open("/hello.txt")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer r.Close()
	data, _ := io.ReadAll(r)
	if string(data) != "hello s3" {
		t.Errorf("content = %q", data)
	}
}

func TestS3MkdirAndReadDir(t *testing.T) {
	b, _ := newTestBackend(t)
	fsys, _ := b.Scope("/")

	if err := fsys.Mkdir("/docs"); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	w, _ := fsys.Create("/docs/a.txt")
	w.Write([]byte("x"))
	w.Close()

	entries, err := fsys.ReadDir("/")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 || !entries[0].IsDir || entries[0].Name != "docs" {
		t.Fatalf("entries = %+v", entries)
	}

	inner, err := fsys.ReadDir("/docs")
	if err != nil {
		t.Fatal(err)
	}
	if len(inner) != 1 || inner[0].Name != "a.txt" || inner[0].IsDir {
		t.Errorf("inner = %+v", inner)
	}
}

func TestS3RemoveDirectory(t *testing.T) {
	b, f := newTestBackend(t)
	fsys, _ := b.Scope("/")

	fsys.Mkdir("/docs")
	w, _ := fsys.Create("/docs/a.txt")
	w.Write([]byte("x"))
	w.Close()

	if err := fsys.Remove("/docs"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	f.mu.RLock()
	remaining := len(f.objects)
	f.mu.RUnlock()
	if remaining != 0 {
		t.Errorf("%d objects left behind", remaining)
	}
}

func TestS3Rename(t *testing.T) {
	b, _ := newTestBackend(t)
	fsys, _ := b.Scope("/")
	w, _ := fsys.Create("/a.txt")
	w.Write([]byte("data"))
	w.Close()

	if err := fsys.Rename("/a.txt", "/b.txt"); err != nil {
		t.Fatalf("Rename: %v", err)
	}
	if fsys.Exists("/a.txt") {
		t.Error("source still exists")
	}
	r, err := fsys.Open("/b.txt")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer r.Close()
	data, _ := io.ReadAll(r)
	if string(data) != "data" {
		t.Errorf("content = %q", data)
	}
}

func TestS3RenameRefusesOverwrite(t *testing.T) {
	b, _ := newTestBackend(t)
	fsys, _ := b.Scope("/")
	for _, name := range []string{"/a.txt", "/b.txt"} {
		w, _ := fsys.Create(name)
		w.Write([]byte("x"))
		w.Close()
	}
	if err := fsys.Rename("/a.txt", "/b.txt"); err != storage.ErrExists {
		t.Fatalf("got %v, want ErrExists", err)
	}
}

func TestS3CopyDirectory(t *testing.T) {
	b, _ := newTestBackend(t)
	fsys, _ := b.Scope("/")
	fsys.Mkdir("/src")
	w, _ := fsys.Create("/src/a.txt")
	w.Write([]byte("payload"))
	w.Close()

	if err := fsys.Copy("/src", "/dst"); err != nil {
		t.Fatalf("Copy: %v", err)
	}
	if !fsys.Exists("/dst/a.txt") {
		t.Error("nested file not copied")
	}
}

func TestS3ScopeConfinement(t *testing.T) {
	b, _ := newTestBackend(t)
	fsys, _ := b.Scope("/")
	fsys.Mkdir("/team/alice")
	fsys.Mkdir("/team/bob")
	w, _ := fsys.Create("/team/bob/secret.txt")
	w.Write([]byte("bob"))
	w.Close()

	alice, err := b.Scope("/team/alice")
	if err != nil {
		t.Fatalf("Scope: %v", err)
	}
	if _, err := alice.Open("/../bob/secret.txt"); err == nil {
		t.Error("alice read bob's file")
	}
	if _, err := alice.ReadDir("/.."); err == nil {
		t.Error("alice listed outside her home")
	}
	if err := alice.Remove("/../bob/secret.txt"); err == nil {
		t.Error("alice removed bob's file")
	}
}

func TestS3ChmodUnsupported(t *testing.T) {
	b, _ := newTestBackend(t)
	fsys, _ := b.Scope("/")
	w, _ := fsys.Create("/a.txt")
	w.Close()
	if err := fsys.Chmod("/a.txt", 0o600); err != storage.ErrUnsupported {
		t.Fatalf("got %v, want ErrUnsupported", err)
	}
	if b.Capabilities().Chmod {
		t.Error("s3 must report chmod as unsupported")
	}
	if b.Capabilities().ServerBrowse {
		t.Error("s3 must not offer the host folder picker")
	}
}

func TestS3EnsureDir(t *testing.T) {
	b, _ := newTestBackend(t)
	if err := b.EnsureDir("/users/alice"); err != nil {
		t.Fatalf("EnsureDir: %v", err)
	}
	if _, err := b.Scope("/users/alice"); err != nil {
		t.Errorf("created home not usable: %v", err)
	}
	if err := b.EnsureDir("/users/alice"); err != storage.ErrExists {
		t.Fatalf("got %v, want ErrExists", err)
	}
}

func TestS3MissingHome(t *testing.T) {
	b, _ := newTestBackend(t)
	if _, err := b.Scope("/nope"); err != storage.ErrNotFound {
		t.Fatalf("got %v, want ErrNotFound", err)
	}
}

func TestS3PrefixIsolatesRepositories(t *testing.T) {
	f, srv := newFakeS3(t)
	mk := func(prefix string) *Backend {
		b, err := New(config.S3{
			Bucket: "testbucket", Region: "us-east-1", Endpoint: srv.URL,
			AccessKey: "k", SecretKey: "s", UsePathStyle: true, Prefix: prefix, Timeout: 5,
		})
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		return b
	}
	a := mk("team-a")
	bk := mk("team-b")

	fa, _ := a.Scope("/")
	w, _ := fa.Create("/only-a.txt")
	w.Write([]byte("a"))
	w.Close()

	fb, _ := bk.Scope("/")
	if fb.Exists("/only-a.txt") {
		t.Error("prefix b sees prefix a's object")
	}
	f.mu.RLock()
	total := len(f.objects)
	f.mu.RUnlock()
	if total != 1 {
		t.Errorf("expected one object, got %d", total)
	}
}

func TestS3RequestsAreSigned(t *testing.T) {
	b, f := newTestBackend(t)
	fsys, _ := b.Scope("/")
	w, _ := fsys.Create("/x.txt")
	w.Write([]byte("y"))
	w.Close()

	f.mu.RLock()
	defer f.mu.RUnlock()
	if len(f.requests) == 0 {
		t.Fatal("no requests recorded")
	}
	for _, r := range f.requests {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "AWS4-HMAC-SHA256 Credential=") {
			t.Errorf("unsigned request: %s %s (%q)", r.Method, r.URL.Path, auth)
		}
		if r.Header.Get("X-Amz-Date") == "" {
			t.Errorf("missing X-Amz-Date on %s", r.URL.Path)
		}
		if r.Header.Get("X-Amz-Content-Sha256") == "" {
			t.Errorf("missing payload hash on %s", r.URL.Path)
		}
	}
}

func TestS3NewRejectsBadConfig(t *testing.T) {
	if _, err := New(config.S3{}); err == nil {
		t.Error("empty config accepted")
	}
	if _, err := New(config.S3{Bucket: "b"}); err == nil {
		t.Error("missing credentials accepted")
	}
	if _, err := New(config.S3{Bucket: "b", AccessKey: "k", SecretKey: "s", Endpoint: "ftp://x"}); err == nil {
		t.Error("non-http endpoint accepted")
	}
}

func TestS3NewFailsFastOnBadBucket(t *testing.T) {
	f, srv := newFakeS3(t)
	f.failWith = http.StatusForbidden
	_, err := New(config.S3{
		Bucket: "b", Region: "us-east-1", Endpoint: srv.URL,
		AccessKey: "k", SecretKey: "s", UsePathStyle: true, Timeout: 5,
	})
	if err == nil {
		t.Fatal("expected an error for an unreadable bucket")
	}
	if !strings.Contains(err.Error(), "cannot read bucket") {
		t.Errorf("unhelpful error: %v", err)
	}
}

func TestS3Description(t *testing.T) {
	b, _ := newTestBackend(t)
	if !strings.HasPrefix(b.Description(), "s3://testbucket") {
		t.Errorf("description = %q", b.Description())
	}
}

func TestS3WriterLimit(t *testing.T) {
	w := &writer{client: &client{}}
	// Simulate a buffer near the limit without allocating it.
	w.buf.WriteString(strings.Repeat("a", 1))
	if _, err := w.Write(make([]byte, 1024)); err != nil {
		t.Errorf("small write failed: %v", err)
	}
}
