package security

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewTokenIsRandom(t *testing.T) {
	a, err := NewToken()
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewToken()
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatal("tokens repeat")
	}
	if len(a) < 40 {
		t.Errorf("token too short: %d chars", len(a))
	}
}

func TestTokenIssuesWhenMissing(t *testing.T) {
	c := NewCSRF(false)
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	token, fresh, err := c.Token(r)
	if err != nil {
		t.Fatal(err)
	}
	if !fresh || token == "" {
		t.Fatalf("token = %q fresh = %v", token, fresh)
	}
}

func TestTokenReusesExisting(t *testing.T) {
	c := NewCSRF(false)
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: c.CookieName, Value: "existing"})
	token, fresh, err := c.Token(r)
	if err != nil {
		t.Fatal(err)
	}
	if fresh {
		t.Error("should not reissue")
	}
	if token != "existing" {
		t.Errorf("token = %q", token)
	}
}

func TestCookieIsReadableByJS(t *testing.T) {
	c := NewCSRF(false)
	cookie := c.Cookie("abc")
	if cookie.HttpOnly {
		t.Error("csrf cookie must not be HttpOnly")
	}
	if cookie.Path != "/" {
		t.Errorf("Path = %q", cookie.Path)
	}
	if cookie.Secure {
		t.Error("Secure should follow configuration")
	}
	if !NewCSRF(true).Cookie("x").Secure {
		t.Error("Secure flag not applied")
	}
}

func TestValidRequiresBoth(t *testing.T) {
	c := NewCSRF(false)
	const token = "token123"

	newReq := func(cookie, header string) *http.Request {
		r := httptest.NewRequest(http.MethodPost, "/", nil)
		if cookie != "" {
			r.AddCookie(&http.Cookie{Name: c.CookieName, Value: cookie})
		}
		if header != "" {
			r.Header.Set(c.Header, header)
		}
		return r
	}

	cases := []struct {
		name        string
		cookie, hdr string
		wantErr     bool
	}{
		{"match", token, token, false},
		{"no cookie", "", token, true},
		{"no header", token, "", true},
		{"mismatch", token, "other", true},
		{"empty both", "", "", true},
	}
	for _, tc := range cases {
		err := c.Valid(newReq(tc.cookie, tc.hdr))
		if tc.wantErr && !errors.Is(err, ErrCSRF) {
			t.Errorf("%s: got %v, want ErrCSRF", tc.name, err)
		}
		if !tc.wantErr && err != nil {
			t.Errorf("%s: unexpected error %v", tc.name, err)
		}
	}
}

func TestCSRFHeaderName(t *testing.T) {
	if NewCSRF(false).Header != "X-CSRF-Token" {
		t.Errorf("Header = %q", NewCSRF(false).Header)
	}
}
