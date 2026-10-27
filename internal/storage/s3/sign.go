package s3

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// sha256Empty is the hash of an empty body, used for unsigned payloads.
const sha256Empty = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

// amzDateLayout is the timestamp format SigV4 requires.
const amzDateLayout = "20060102T150405Z"

// signer applies AWS Signature Version 4 to requests.
//
// The algorithm is fixed by the S3 specification: build a canonical request,
// hash it, build a string to sign, derive a signing key from the secret, and
// add an Authorization header. See
// https://docs.aws.amazon.com/general/latest/gr/sigv4_signing.html
type signer struct {
	accessKey string
	secretKey string
	region    string
	service   string
	// now is overridable so tests can produce deterministic signatures.
	now func() time.Time
}

// sign adds the required headers and returns an error only if the request
// cannot be signed (for example a malformed query).
func (s *signer) sign(req *http.Request, payloadHash string) {
	now := s.now().UTC()
	amzDate := now.Format(amzDateLayout)
	dateStamp := now.Format("20060102")

	req.Header.Set("X-Amz-Date", amzDate)
	req.Header.Set("X-Amz-Content-Sha256", payloadHash)

	// Host is taken from the request URL and must be included in the signature.
	host := req.URL.Host
	if host == "" {
		host = req.Host
	}

	canonicalHeaders, signedHeaders := canonicalHeaders(req, host)
	canonicalRequest := strings.Join([]string{
		req.Method,
		canonicalURI(req.URL),
		canonicalQuery(req.URL),
		canonicalHeaders,
		signedHeaders,
		payloadHash,
	}, "\n")

	scope := dateStamp + "/" + s.region + "/" + s.service + "/aws4_request"
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzDate,
		scope,
		hashHex([]byte(canonicalRequest)),
	}, "\n")

	key := s.signingKey(dateStamp)
	signature := hex.EncodeToString(hmacSHA256(key, []byte(stringToSign)))

	req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential="+s.accessKey+"/"+scope+
		", SignedHeaders="+signedHeaders+", Signature="+signature)
}

// canonicalHeaders returns the canonical header block and the signed header
// list, both sorted by lower-case name.
func canonicalHeaders(req *http.Request, host string) (string, string) {
	headers := map[string]string{"host": host}

	// Only a fixed set of headers is signed. Signing everything would break
	// when a transport adds headers afterwards.
	for _, name := range []string{"Content-Type", "X-Amz-Date", "X-Amz-Content-Sha256", "X-Amz-Copy-Source"} {
		if v := req.Header.Get(name); v != "" {
			headers[strings.ToLower(name)] = strings.TrimSpace(v)
		}
	}

	names := make([]string, 0, len(headers))
	for name := range headers {
		names = append(names, name)
	}
	sort.Strings(names)

	var block strings.Builder
	for _, name := range names {
		block.WriteString(name)
		block.WriteString(":")
		block.WriteString(headers[name])
		block.WriteString("\n")
	}
	return block.String(), strings.Join(names, ";")
}

// canonicalURI returns the URI-encoded path, keeping the separators.
func canonicalURI(u *url.URL) string {
	if u.Path == "" {
		return "/"
	}
	return uriEncode(u.Path, false)
}

// canonicalQuery returns the sorted, encoded query string.
func canonicalQuery(u *url.URL) string {
	if u.RawQuery == "" {
		return ""
	}
	parts := strings.Split(u.RawQuery, "&")
	type pair struct{ key, value string }
	pairs := make([]pair, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		k, v, _ := strings.Cut(part, "=")
		decodedKey, err := url.QueryUnescape(k)
		if err != nil {
			decodedKey = k
		}
		decodedValue, err := url.QueryUnescape(v)
		if err != nil {
			decodedValue = v
		}
		pairs = append(pairs, pair{uriEncode(decodedKey, true), uriEncode(decodedValue, true)})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].key != pairs[j].key {
			return pairs[i].key < pairs[j].key
		}
		return pairs[i].value < pairs[j].value
	})
	var out strings.Builder
	for i, p := range pairs {
		if i > 0 {
			out.WriteString("&")
		}
		out.WriteString(p.key)
		out.WriteString("=")
		out.WriteString(p.value)
	}
	return out.String()
}

// signingKey derives the SigV4 signing key for a date.
func (s *signer) signingKey(dateStamp string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+s.secretKey), []byte(dateStamp))
	kRegion := hmacSHA256(kDate, []byte(s.region))
	kService := hmacSHA256(kRegion, []byte(s.service))
	return hmacSHA256(kService, []byte("aws4_request"))
}

// uriEncode percent-encodes a string per the SigV4 rules: everything except
// unreserved characters, and "/" only when encodeSlash is false.
func uriEncode(s string, encodeSlash bool) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9',
			c == '-', c == '_', c == '.', c == '~':
			b.WriteByte(c)
		case c == '/' && !encodeSlash:
			b.WriteByte(c)
		default:
			b.WriteString("%")
			b.WriteString(strings.ToUpper(hex.EncodeToString([]byte{c})))
		}
	}
	return b.String()
}

// hmacSHA256 returns the HMAC of data with key.
func hmacSHA256(key, data []byte) []byte {
	m := hmac.New(sha256.New, key)
	m.Write(data)
	return m.Sum(nil)
}

// hashHex returns the hex SHA-256 of data.
func hashHex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
