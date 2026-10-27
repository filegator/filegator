package security

import (
	"fmt"
	"net/http"
	"strings"
)

// DefaultHeaders are the response headers added to every response unless the
// administrator replaces them.
//
// X-Frame-Options blocks clickjacking, X-Content-Type-Options stops MIME
// sniffing (important because user-uploaded files are served from this
// origin), and Referrer-Policy avoids leaking URLs.
func DefaultHeaders() map[string]string {
	return map[string]string{
		"X-Content-Type-Options":     "nosniff",
		"X-Frame-Options":            "SAMEORIGIN",
		"Referrer-Policy":            "same-origin",
		"Cross-Origin-Opener-Policy": "same-origin",
	}
}

// Headers adds the given response headers to every response. Keys are header
// names; values may be empty to send the header with no value.
//
// A header whose value is removed by the administrator is simply not sent.
// Callers that want the defaults merged with custom values should start from
// DefaultHeaders and apply their overrides.
func Headers(configured map[string]string) func(http.Handler) http.Handler {
	// Copy once so later config mutations cannot change behaviour mid-flight.
	headers := make(map[string]string, len(configured))
	for k, v := range configured {
		headers[k] = v
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			for k, v := range headers {
				if v == "" {
					// Explicit empty value means "do not send this header".
					continue
				}
				h.Set(k, v)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ValidateHeaders checks that every configured header name and value is safe
// to send. Rejecting CR/LF prevents response splitting.
func ValidateHeaders(headers map[string]string) error {
	for name, value := range headers {
		if name == "" {
			return fmt.Errorf("security: header name must not be empty")
		}
		for _, r := range name {
			if !isHeaderTokenRune(r) {
				return fmt.Errorf("security: invalid header name %q", name)
			}
		}
		if strings.ContainsAny(value, "\r\n\x00") {
			return fmt.Errorf("security: header %q contains invalid characters", name)
		}
	}
	return nil
}

// isHeaderTokenRune reports whether r may appear in an HTTP header name.
func isHeaderTokenRune(r rune) bool {
	switch {
	case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		return true
	}
	return strings.ContainsRune("!#$%&'*+-.^_`|~", r)
}

// MergeHeaders returns defaults overridden by custom values. A key in custom
// with an empty value removes that header.
func MergeHeaders(defaults, custom map[string]string) map[string]string {
	out := make(map[string]string, len(defaults)+len(custom))
	for k, v := range defaults {
		out[k] = v
	}
	for k, v := range custom {
		out[k] = v
	}
	return out
}
