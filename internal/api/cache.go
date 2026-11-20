package api

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/filegator/filegator/internal/storage"
)

// noStore keeps a one-shot download out of every cache.
//
// Attachments and batch zips are saved or consumed once, and a file replaced
// at the same path must never be shown from a cache. Inline responses are
// different: they are opened repeatedly by a gallery or a PDF preview, so
// setInlineCache lets the browser keep the body and ask before every reuse.
func noStore(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
}

// inlineETag returns a weak validator for an inline representation.
//
// The file body is not read. The validator is built from what the backend
// already reported (S3's object ETag where it has one, size and modification
// time otherwise) plus the content type, because changing download_inline
// changes the representation without touching the file. A content hash would
// be exact but would read the whole file on every request, which is the cost
// this revalidation exists to avoid.
//
// The user scope is part of the hash because the same URL means different
// files for different homes. A browser's private cache is keyed by URL, not by
// session: without the scope, one user's validator could answer a second
// user's request with a 304 and show the first user's bytes.
func inlineETag(e *storage.Entry, contentType, scope string) string {
	base := e.ETag
	if base == "" {
		base = strconv.FormatInt(e.Size, 10) + "-" + strconv.FormatInt(modTimeNanos(e.Modified), 10)
	}
	sum := sha256.Sum256([]byte("v1|" + scope + "|" + base + "|" + contentType))
	return fmt.Sprintf(`W/"%x-%x"`, e.Size, sum[:16])
}

// modTimeNanos keeps a zero modification time from looking like a date before
// the epoch in the validator.
func modTimeNanos(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UnixNano()
}

// setInlineCache marks an inline response as private to the browser and
// revalidated before every use. It needs the etag inlineETag already produced
// so a 304 path and a 200 path carry exactly the same validator.
func setInlineCache(w http.ResponseWriter, e *storage.Entry, etag string) {
	w.Header().Set("Cache-Control", "private, no-cache")
	// A browser cache is keyed by URL, not by session. The ETag is scoped to
	// the user as well, but Vary keeps a cache that ignores that scope from
	// even considering another session's copy.
	w.Header().Set("Vary", "Cookie")
	w.Header().Set("ETag", etag)
	if !e.Modified.IsZero() {
		w.Header().Set("Last-Modified", e.Modified.UTC().Format(http.TimeFormat))
	}
}

// notModified reports whether the request's preconditions say the cached copy
// is still current.
//
// If-None-Match takes precedence over If-Modified-Since (RFC 7232 section
// 3.3), so the date is only consulted when the header is absent.
func notModified(r *http.Request, etag string, modtime time.Time) bool {
	if inm := r.Header.Get("If-None-Match"); inm != "" {
		return etagMatches(inm, etag)
	}
	ims := r.Header.Get("If-Modified-Since")
	if ims == "" || modtime.IsZero() {
		return false
	}
	t, err := http.ParseTime(ims)
	if err != nil {
		return false
	}
	return !modtime.UTC().Truncate(time.Second).After(t)
}

// etagMatches applies the weak comparison HTTP cache validation uses: W/ is
// ignored on both sides, and "*" matches any current representation.
func etagMatches(header, etag string) bool {
	for _, candidate := range strings.Split(header, ",") {
		candidate = strings.TrimSpace(candidate)
		if candidate == "*" {
			return true
		}
		if weakETag(candidate) == weakETag(etag) {
			return true
		}
	}
	return false
}

func weakETag(v string) string {
	return strings.TrimPrefix(strings.TrimSpace(v), "W/")
}
