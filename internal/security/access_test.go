package security

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"
)

func TestIPListExactAndCIDR(t *testing.T) {
	l, err := NewIPList([]string{"127.0.0.1", "10.0.0.0/8", " 2001:db8::/32 ", ""})
	if err != nil {
		t.Fatalf("NewIPList: %v", err)
	}
	cases := map[string]bool{
		"127.0.0.1":   true,
		"127.0.0.2":   false,
		"10.1.2.3":    true,
		"11.0.0.1":    false,
		"2001:db8::1": true,
		"2001:db9::1": false,
	}
	for ip, want := range cases {
		if got := l.Contains(netip.MustParseAddr(ip)); got != want {
			t.Errorf("Contains(%s) = %v, want %v", ip, got, want)
		}
	}
}

func TestIPListInvalid(t *testing.T) {
	for _, bad := range []string{"not-an-ip", "10.0.0.0/99", "1.2.3"} {
		if _, err := NewIPList([]string{bad}); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestIPListIPv4MappedIPv6(t *testing.T) {
	l, _ := NewIPList([]string{"192.168.1.0/24"})
	// ::ffff:192.168.1.5 must match the IPv4 rule.
	if !l.Contains(netip.MustParseAddr("::ffff:192.168.1.5")) {
		t.Error("IPv4-mapped address did not match")
	}
}

func TestEmptyListMatchesNothing(t *testing.T) {
	l, err := NewIPList(nil)
	if err != nil {
		t.Fatal(err)
	}
	if !l.Empty() {
		t.Error("should be empty")
	}
	if l.Contains(netip.MustParseAddr("1.2.3.4")) {
		t.Error("empty list matched")
	}
}

func newIPRequest(remote, xff string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = remote
	if xff != "" {
		r.Header.Set("X-Forwarded-For", xff)
	}
	return r
}

func TestIPFilterDenyWins(t *testing.T) {
	f, err := NewIPFilter([]string{"10.0.0.0/8"}, []string{"10.1.1.1"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if f.Allow(newIPRequest("10.1.1.1:1234", "")) {
		t.Error("deny list should win over allow list")
	}
	if !f.Allow(newIPRequest("10.2.2.2:1234", "")) {
		t.Error("allowed address rejected")
	}
	if f.Allow(newIPRequest("8.8.8.8:1234", "")) {
		t.Error("address outside allow list accepted")
	}
}

func TestIPFilterOpenWhenAllowEmpty(t *testing.T) {
	f, err := NewIPFilter(nil, []string{"6.6.6.6"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if !f.Allow(newIPRequest("1.2.3.4:1", "")) {
		t.Error("should allow when only a deny list is set")
	}
	if f.Allow(newIPRequest("6.6.6.6:1", "")) {
		t.Error("denied address accepted")
	}
}

func TestIPFilterIgnoresForwardedHeaderByDefault(t *testing.T) {
	f, _ := NewIPFilter([]string{"1.2.3.4"}, nil, "")
	// Spoofed X-Forwarded-For must not grant access when no header is trusted.
	if f.Allow(newIPRequest("9.9.9.9:1", "1.2.3.4")) {
		t.Error("spoofed X-Forwarded-For honoured")
	}
}

func TestIPFilterTrustedHeader(t *testing.T) {
	newReq := func(remote string, headers map[string]string) *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = remote
		for k, v := range headers {
			r.Header.Set(k, v)
		}
		return r
	}

	t.Run("cloudflare", func(t *testing.T) {
		f, err := NewIPFilter([]string{"203.0.113.5"}, nil, "CF-Connecting-IP")
		if err != nil {
			t.Fatal(err)
		}
		// Proxy address differs from the client address.
		if !f.Allow(newReq("172.16.0.1:9000", map[string]string{"CF-Connecting-IP": "203.0.113.5"})) {
			t.Error("trusted header not honoured")
		}
		// Untrusted header of a different name is ignored.
		if f.Allow(newReq("172.16.0.1:9000", map[string]string{"X-Forwarded-For": "203.0.113.5"})) {
			t.Error("unrelated header honoured")
		}
	})

	t.Run("x-real-ip", func(t *testing.T) {
		f, _ := NewIPFilter(nil, []string{"10.9.9.9"}, "X-Real-IP")
		if f.Allow(newReq("172.16.0.1:9000", map[string]string{"X-Real-IP": "10.9.9.9"})) {
			t.Error("deny list not applied to trusted header")
		}
	})

	t.Run("xff_chain", func(t *testing.T) {
		f, _ := NewIPFilter([]string{"198.51.100.7"}, nil, "X-Forwarded-For")
		if !f.Allow(newReq("172.16.0.1:9000", map[string]string{"X-Forwarded-For": "198.51.100.7, 10.0.0.1"})) {
			t.Error("left-most X-Forwarded-For address not used")
		}
	})

	t.Run("falls_back_to_remote_addr", func(t *testing.T) {
		f, _ := NewIPFilter([]string{"192.0.2.1"}, nil, "CF-Connecting-IP")
		// Missing header: RemoteAddr decides, not "allow everything".
		if f.Allow(newReq("192.0.2.1:1", nil)) != true {
			t.Error("RemoteAddr not used as fallback")
		}
		if f.Allow(newReq("192.0.2.2:1", nil)) {
			t.Error("RemoteAddr not used as fallback")
		}
	})

	t.Run("invalid_header_value_falls_back", func(t *testing.T) {
		f, _ := NewIPFilter([]string{"192.0.2.1"}, nil, "CF-Connecting-IP")
		r := newReq("192.0.2.2:1", map[string]string{"CF-Connecting-IP": "not-an-ip"})
		if f.Allow(r) {
			t.Error("invalid header value should fall back to RemoteAddr")
		}
	})
}

func TestClientIPFromHeader(t *testing.T) {
	r := newIPRequest("9.9.9.9:1", "")
	r.Header.Set("CF-Connecting-IP", "1.2.3.4")
	if addr, _ := ClientIPFromHeader(r, "CF-Connecting-IP"); addr.String() != "1.2.3.4" {
		t.Errorf("got %s", addr)
	}
	if addr, _ := ClientIPFromHeader(r, ""); addr.String() != "9.9.9.9" {
		t.Errorf("empty header should use RemoteAddr, got %s", addr)
	}
}

func TestClientIPInvalid(t *testing.T) {
	r := newIPRequest("not-an-ip", "")
	if _, ok := ClientIP(r); ok {
		t.Error("invalid address parsed")
	}
	f, _ := NewIPFilter([]string{"1.2.3.4"}, nil, "")
	if f.Allow(r) {
		t.Error("unparseable address allowed against a non-empty allow list")
	}
}

func TestLockoutThreshold(t *testing.T) {
	l := NewLockout(3, time.Minute)
	const ip = "1.1.1.1"
	for i := 1; i <= 2; i++ {
		if l.Fail(ip) {
			t.Fatalf("blocked after %d attempts", i)
		}
		if l.Blocked(ip) {
			t.Fatalf("blocked after %d attempts", i)
		}
	}
	if !l.Fail(ip) {
		t.Fatal("should block on the 3rd attempt")
	}
	if !l.Blocked(ip) {
		t.Fatal("should be blocked")
	}
	if l.Blocked("2.2.2.2") {
		t.Error("other address blocked")
	}
}

func TestLockoutReset(t *testing.T) {
	l := NewLockout(2, time.Minute)
	l.Fail("x")
	l.Reset("x")
	l.Fail("x")
	if l.Blocked("x") {
		t.Error("Reset did not clear failures")
	}
}

func TestLockoutExpiry(t *testing.T) {
	l := NewLockout(2, time.Minute)
	now := time.Now()
	l.now = func() time.Time { return now }
	l.Fail("x")
	l.Fail("x")
	if !l.Blocked("x") {
		t.Fatal("should be blocked")
	}
	now = now.Add(2 * time.Minute)
	if l.Blocked("x") {
		t.Error("lockout did not expire")
	}
}

func TestLockoutDisabled(t *testing.T) {
	l := NewLockout(0, time.Minute)
	for i := 0; i < 10; i++ {
		l.Fail("x")
	}
	if l.Blocked("x") {
		t.Error("lockout should be disabled")
	}
}

func TestLockoutSweep(t *testing.T) {
	l := NewLockout(5, time.Minute)
	now := time.Now()
	l.now = func() time.Time { return now }

	// Fill past the sweep threshold with distinct keys.
	for i := 0; i < 400; i++ {
		l.Fail(fmt.Sprintf("key-%d", i))
	}
	if len(l.entries) != 400 {
		t.Fatalf("entries = %d, want 400", len(l.entries))
	}

	// After the timeout the next failure should drop the expired entries.
	now = now.Add(2 * time.Minute)
	l.Fail("fresh")
	if len(l.entries) > 2 {
		t.Errorf("expired entries were not swept: %d remain", len(l.entries))
	}
}
