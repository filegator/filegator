// Package sso implements single sign-on handoffs from other applications.
//
// Another application, typically a CMS or admin panel, signs a short URL with
// a secret it shares with FileGator. Following that URL signs the visitor in
// and redirects them to the file browser. No password crosses the boundary and
// nothing is stored server-side except the nonces already spent.
package sso

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/filegator/filegator/internal/config"
)

// Errors returned by this package. They are deliberately indistinguishable to
// the caller: an unknown application and a bad signature must look the same,
// so a handoff URL cannot be used to probe which applications exist.
var (
	// ErrInvalid is returned for any handoff that cannot be trusted.
	ErrInvalid = errors.New("sso: invalid handoff")
	// ErrExpired is returned when the handoff is past its expiry.
	ErrExpired = errors.New("sso: handoff expired")
	// ErrReplayed is returned when the nonce has already been used.
	ErrReplayed = errors.New("sso: handoff already used")
	// ErrUnknownUser is returned in "existing" mode for a name with no account.
	ErrUnknownUser = errors.New("sso: unknown user")
)

// maxNonces bounds the replay cache so a hostile client cannot exhaust memory
// by requesting handoffs it never completes. Expired entries are dropped on
// every check, so the cache only holds handoffs still within their window.
const maxNonces = 10000

// Handoff is a verified login request.
type Handoff struct {
	// Application is the config entry that authorised it.
	Application config.SSOApplication
	// Username is the account the visitor claims to be.
	Username string
	// Nonce is the single-use value that was spent.
	Nonce string
}

// Verifier checks handoffs and remembers which have been used.
type Verifier struct {
	// now is overridable in tests.
	now func() time.Time

	mu        sync.Mutex
	used      map[string]time.Time // nonce => expiry
	lastSweep time.Time
}

// NewVerifier returns a verifier with an empty replay cache.
func NewVerifier() *Verifier {
	return &Verifier{now: time.Now, used: make(map[string]time.Time)}
}

// Signature returns the signature a handoff URL must carry.
//
// The fields are joined with "|" and signed with the application's secret, so
// any change to a field invalidates it. Exported so a test — and the
// documentation — can show the exact algorithm the other application must
// reproduce.
func Signature(secret, username, application string, expiry int64, nonce string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(strings.Join([]string{username, application, strconv.FormatInt(expiry, 10), nonce}, "|")))
	return hex.EncodeToString(mac.Sum(nil))
}

// Verify checks a handoff and, if it is valid, spends its nonce.
//
// The nonce is spent even when the user turns out to be unknown, so a URL is
// never valid twice regardless of what happens afterwards.
func (v *Verifier) Verify(cfg config.SSO, username, application string, expiry int64, nonce, signature string) (*Handoff, error) {
	username = strings.TrimSpace(username)
	nonce = strings.TrimSpace(nonce)
	if username == "" || nonce == "" || len(nonce) > 128 {
		return nil, ErrInvalid
	}
	app := cfg.FindSSOApplication(application)
	if app == nil {
		return nil, ErrInvalid
	}

	// Signatures are compared in constant time so a correct prefix cannot be
	// discovered by timing.
	want := Signature(app.Secret, username, application, expiry, nonce)
	if !hmac.Equal([]byte(strings.ToLower(signature)), []byte(want)) {
		return nil, ErrInvalid
	}

	now := v.now()
	if expiry <= now.Unix() {
		return nil, ErrExpired
	}

	if !v.spend(nonce, time.Unix(expiry, 0)) {
		return nil, ErrReplayed
	}
	return &Handoff{Application: *app, Username: username, Nonce: nonce}, nil
}

// spend records a nonce, reporting false when it was already used.
func (v *Verifier) spend(nonce string, expires time.Time) bool {
	v.mu.Lock()
	defer v.mu.Unlock()

	now := v.now()
	// Drop expired nonces occasionally rather than on every call; the cache
	// holds at most one entry per handoff within its short window.
	if now.Sub(v.lastSweep) > time.Minute || len(v.used) > maxNonces {
		for n, exp := range v.used {
			if now.After(exp) {
				delete(v.used, n)
			}
		}
		v.lastSweep = now
	}
	if _, seen := v.used[nonce]; seen {
		return false
	}
	if len(v.used) >= maxNonces {
		// Refuse rather than grow without bound. Reaching this means something
		// is generating handoffs far faster than they are used.
		return false
	}
	v.used[nonce] = expires
	return true
}

// Expiry returns the lifetime to advertise for an application.
func Expiry(app config.SSOApplication) time.Duration {
	seconds := app.Expiry
	if seconds <= 0 {
		seconds = config.DefaultSSOExpiry
	}
	return time.Duration(seconds) * time.Second
}
