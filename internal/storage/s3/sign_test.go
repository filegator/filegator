package s3

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

// TestSignerAWSVector reproduces the worked example from the AWS S3
// documentation: GET /test.txt on examplebucket in us-east-1 at
// 2013-05-24T00:00:00Z. The expected signature is published by AWS; if this
// fails, real requests would be rejected.
func TestSignerAWSVector(t *testing.T) {
	s := &signer{
		accessKey: "AKIAIOSFODNN7EXAMPLE",
		secretKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		region:    "us-east-1",
		service:   "s3",
		now:       func() time.Time { return time.Date(2013, 5, 24, 0, 0, 0, 0, time.UTC) },
	}

	u, err := url.Parse("https://examplebucket.s3.amazonaws.com/test.txt")
	if err != nil {
		t.Fatal(err)
	}
	req := &http.Request{
		Method: http.MethodGet,
		URL:    u,
		Header: make(http.Header),
		Host:   "examplebucket.s3.amazonaws.com",
	}

	s.sign(req, sha256Empty)

	const wantSig = "df548e2ce037944d03f3e68682813b093763996d597cf890ca3d9037fd231eb4"
	auth := req.Header.Get("Authorization")
	if !strings.HasSuffix(auth, "Signature="+wantSig) {
		t.Errorf("Authorization =\n  %s\nwant signature %s", auth, wantSig)
	}
	if got := req.Header.Get("X-Amz-Date"); got != "20130524T000000Z" {
		t.Errorf("X-Amz-Date = %q", got)
	}
}

func TestCanonicalQuerySorting(t *testing.T) {
	u, _ := url.Parse("https://h/?b=2&a=1&B=3&a=0")
	got := canonicalQuery(u)
	want := "B=3&a=0&a=1&b=2"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestCanonicalQueryEscaping(t *testing.T) {
	u, _ := url.Parse("https://h/?prefix=a/b c&list-type=2")
	got := canonicalQuery(u)
	if !strings.Contains(got, "prefix=a%2Fb%20c") {
		t.Errorf("slash or space not encoded: %q", got)
	}
	if !strings.Contains(got, "list-type=2") {
		t.Errorf("dash should not be encoded: %q", got)
	}
}

func TestURIEncode(t *testing.T) {
	cases := map[string]string{
		"abc":     "abc",
		"a b":     "a%20b",
		"a/b":     "a%2Fb",
		"a~b":     "a~b",
		"a.b-c_d": "a.b-c_d",
		"ümlaut":  "%C3%BCmlaut",
		"a+b":     "a%2Bb",
	}
	for in, want := range cases {
		if got := uriEncode(in, true); got != want {
			t.Errorf("uriEncode(%q) = %q, want %q", in, got, want)
		}
	}
	// Paths keep their separators.
	if got := uriEncode("/a/b", false); got != "/a/b" {
		t.Errorf("path encoding = %q", got)
	}
}

func TestCanonicalURIEncodesPath(t *testing.T) {
	u, _ := url.Parse("https://h/bucket/a file.txt")
	if got := canonicalURI(u); got != "/bucket/a%20file.txt" {
		t.Errorf("got %q", got)
	}
}

func TestSigningKeyIsDeterministic(t *testing.T) {
	s := &signer{secretKey: "secret", region: "eu-west-1", service: "s3"}
	a := s.signingKey("20240101")
	b := s.signingKey("20240101")
	if string(a) != string(b) {
		t.Error("signing key is not deterministic")
	}
	if string(a) == string(s.signingKey("20240102")) {
		t.Error("signing key should vary by date")
	}
}
