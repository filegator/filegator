package security

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
	"time"
)

// IPList is an allow/deny list of IP addresses and CIDR ranges.
type IPList struct {
	prefixes []netip.Prefix
}

// NewIPList parses entries such as "127.0.0.1", "10.0.0.0/8" or "2001:db8::/32".
// An empty list matches nothing.
func NewIPList(entries []string) (*IPList, error) {
	l := &IPList{}
	for _, raw := range entries {
		entry := strings.TrimSpace(raw)
		if entry == "" {
			continue
		}
		if strings.Contains(entry, "/") {
			p, err := netip.ParsePrefix(entry)
			if err != nil {
				return nil, err
			}
			l.prefixes = append(l.prefixes, p.Masked())
			continue
		}
		addr, err := netip.ParseAddr(entry)
		if err != nil {
			return nil, err
		}
		addr = addr.Unmap()
		l.prefixes = append(l.prefixes, netip.PrefixFrom(addr, addr.BitLen()))
	}
	return l, nil
}

// Empty reports whether the list has no entries.
func (l *IPList) Empty() bool { return l == nil || len(l.prefixes) == 0 }

// Contains reports whether addr is covered by the list.
func (l *IPList) Contains(addr netip.Addr) bool {
	if l == nil {
		return false
	}
	addr = addr.Unmap()
	for _, p := range l.prefixes {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}

// IPFilter enforces the allow and deny lists. A deny match always wins, so an
// address can never be re-enabled by the allow list.
type IPFilter struct {
	allow  *IPList
	deny   *IPList
	header string
}

// NewIPFilter builds a filter from raw config entries. trustedHeader names the
// header carrying the real client IP behind a proxy; empty means RemoteAddr is
// used.
func NewIPFilter(allow, deny []string, trustedHeader string) (*IPFilter, error) {
	a, err := NewIPList(allow)
	if err != nil {
		return nil, err
	}
	d, err := NewIPList(deny)
	if err != nil {
		return nil, err
	}
	return &IPFilter{allow: a, deny: d, header: trustedHeader}, nil
}

// Allow reports whether the request's client address may proceed. When the
// allow list is non-empty, addresses not on it are rejected.
func (f *IPFilter) Allow(r *http.Request) bool {
	addr, ok := f.ClientIP(r)
	if !ok {
		// An unparseable address cannot be trusted against an allow list.
		return f.allow.Empty()
	}
	if f.deny.Contains(addr) {
		return false
	}
	if f.allow.Empty() {
		return true
	}
	return f.allow.Contains(addr)
}

// Address renders the client address for a log entry.
func (f *IPFilter) Address(r *http.Request) string {
	if addr, ok := f.ClientIP(r); ok {
		return addr.String()
	}
	return r.RemoteAddr
}

// ClientIP returns the client address using the filter's trusted header.
func (f *IPFilter) ClientIP(r *http.Request) (netip.Addr, bool) {
	return ClientIPFromHeader(r, f.header)
}

// ClientIP extracts the client address from r, using only RemoteAddr.
func ClientIP(r *http.Request) (netip.Addr, bool) {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	addr, err := netip.ParseAddr(strings.TrimSpace(host))
	if err != nil {
		return netip.Addr{}, false
	}
	return addr.Unmap(), true
}

// ClientIPFromHeader returns the client address as reported by the given
// header, falling back to RemoteAddr. header is the configured trusted
// header name (for example "CF-Connecting-IP" or "X-Forwarded-For"); an empty
// header always uses RemoteAddr.
//
// For X-Forwarded-For the left-most address is used, since proxies append to
// the list. Only the value is read when the admin has explicitly configured a
// trusted header, so a client cannot spoof it on a direct connection.
func ClientIPFromHeader(r *http.Request, header string) (netip.Addr, bool) {
	if header != "" {
		if raw := r.Header.Get(header); raw != "" {
			// Some proxies send a comma separated chain even for dedicated
			// headers; take the first element in that case.
			first, _, _ := strings.Cut(raw, ",")
			if addr, err := netip.ParseAddr(strings.TrimSpace(first)); err == nil {
				return addr.Unmap(), true
			}
		}
	}
	return ClientIP(r)
}

// Lockout records failed login attempts per client address and blocks further
// attempts once a threshold is reached, mirroring v7's lockout_attempts and
// lockout_timeout settings.
type Lockout struct {
	attempts  int
	timeout   time.Duration
	now       func() time.Time
	entries   map[string]*lockEntry
	sweepEach int
	lastSweep time.Time
}

type lockEntry struct {
	failures int
	expires  time.Time
}

// NewLockout creates a lockout tracker. attempts <= 0 disables locking.
func NewLockout(attempts int, timeout time.Duration) *Lockout {
	return &Lockout{
		attempts:  attempts,
		timeout:   timeout,
		now:       time.Now,
		entries:   make(map[string]*lockEntry),
		sweepEach: 256,
	}
}

// Blocked reports whether the client is currently locked out.
func (l *Lockout) Blocked(key string) bool {
	if l == nil || l.attempts <= 0 {
		return false
	}
	e, ok := l.entries[key]
	if !ok {
		return false
	}
	if l.now().After(e.expires) {
		delete(l.entries, key)
		return false
	}
	return e.failures >= l.attempts
}

// Fail records a failed attempt and returns true when the client just became
// blocked.
func (l *Lockout) Fail(key string) bool {
	if l == nil || l.attempts <= 0 {
		return false
	}
	l.maybeSweep()

	e, ok := l.entries[key]
	if !ok || l.now().After(e.expires) {
		e = &lockEntry{}
		l.entries[key] = e
	}
	e.failures++
	e.expires = l.now().Add(l.timeout)
	return e.failures == l.attempts
}

// Reset clears the failure count for key after a successful login.
func (l *Lockout) Reset(key string) {
	if l == nil {
		return
	}
	delete(l.entries, key)
}

// maybeSweep removes expired entries to bound memory growth.
func (l *Lockout) maybeSweep() {
	if len(l.entries) < l.sweepEach {
		return
	}
	now := l.now()
	if !l.lastSweep.IsZero() && now.Sub(l.lastSweep) < l.timeout {
		return
	}
	for k, e := range l.entries {
		if now.After(e.expires) {
			delete(l.entries, k)
		}
	}
	l.lastSweep = now
}
