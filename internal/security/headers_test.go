package security

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHeadersMiddleware(t *testing.T) {
	h := Headers(DefaultHeaders())(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	for k, v := range DefaultHeaders() {
		if got := rec.Header().Get(k); got != v {
			t.Errorf("%s = %q, want %q", k, got, v)
		}
	}
}

func TestHeadersMiddlewareKeepsBody(t *testing.T) {
	h := Headers(DefaultHeaders())(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("body"))
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if !strings.Contains(rec.Body.String(), "body") {
		t.Errorf("body = %q", rec.Body.String())
	}
}

func TestHeadersCustomAndAdditional(t *testing.T) {
	headers := MergeHeaders(DefaultHeaders(), map[string]string{
		"Strict-Transport-Security": "max-age=31536000; includeSubDomains",
		"Content-Security-Policy":   "default-src 'self'",
		"X-Frame-Options":           "DENY",
	})
	h := Headers(headers)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	cases := map[string]string{
		"Strict-Transport-Security": "max-age=31536000; includeSubDomains",
		"Content-Security-Policy":   "default-src 'self'",
		"X-Frame-Options":           "DENY",
		// Untouched defaults survive the merge.
		"X-Content-Type-Options": "nosniff",
	}
	for k, want := range cases {
		if got := rec.Header().Get(k); got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}
}

func TestHeadersRemoveDefault(t *testing.T) {
	headers := MergeHeaders(DefaultHeaders(), map[string]string{"X-Frame-Options": ""})
	h := Headers(headers)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if _, ok := rec.Header()["X-Frame-Options"]; ok {
		t.Error("empty value should remove the header")
	}
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("other defaults should remain")
	}
}

func TestHeadersSnapshotIsIndependent(t *testing.T) {
	cfg := map[string]string{"X-Test": "one"}
	h := Headers(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	cfg["X-Test"] = "two"

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if got := rec.Header().Get("X-Test"); got != "one" {
		t.Errorf("X-Test = %q, want snapshot value", got)
	}
}

func TestValidateHeaders(t *testing.T) {
	valid := map[string]string{
		"X-Custom":                  "value",
		"Strict-Transport-Security": "max-age=31536000",
		"Content-Security-Policy":   "default-src 'self'; img-src *",
	}
	if err := ValidateHeaders(valid); err != nil {
		t.Errorf("valid headers rejected: %v", err)
	}

	invalid := []map[string]string{
		{"": "value"},
		{"Bad Header": "value"},
		{"X-Ok": "value\r\nInjected: 1"},
		{"X-Ok": "value\x00"},
	}
	for _, h := range invalid {
		if err := ValidateHeaders(h); err == nil {
			t.Errorf("%v accepted", h)
		}
	}
}
