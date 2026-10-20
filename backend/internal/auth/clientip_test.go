package auth

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"
)

func mustTrusted(t *testing.T, s string) []netip.Prefix {
	t.Helper()
	p, err := ParseTrustedProxies(s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return p
}

func reqFrom(remote string, xff ...string) *http.Request {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = remote
	for _, v := range xff {
		r.Header.Add("X-Forwarded-For", v)
	}
	return r
}

func TestClientIP(t *testing.T) {
	trusted := mustTrusted(t, "10.0.0.0/8, 192.168.1.5, ::1")
	cases := []struct {
		name   string
		remote string
		xff    []string
		want   string
	}{
		{"no proxy, no header", "203.0.113.7:5555", nil, "203.0.113.7"},
		{"spoofed XFF from untrusted peer is ignored", "203.0.113.7:5555", []string{"1.2.3.4"}, "203.0.113.7"},
		{"spoofed chain from untrusted peer is ignored", "203.0.113.7:5555", []string{"1.2.3.4, 10.0.0.9"}, "203.0.113.7"},
		{"trusted peer, single hop", "10.1.2.3:80", []string{"198.51.100.4"}, "198.51.100.4"},
		{"trusted peer, right-most untrusted hop wins over spoofed left", "10.1.2.3:80", []string{"6.6.6.6, 198.51.100.4"}, "198.51.100.4"},
		{"trusted peer, skips trusted hops", "10.1.2.3:80", []string{"6.6.6.6, 198.51.100.4, 10.9.9.9"}, "198.51.100.4"},
		{"multiple XFF headers are joined", "192.168.1.5:80", []string{"6.6.6.6", "198.51.100.4"}, "198.51.100.4"},
		{"trusted peer, no header", "10.1.2.3:80", nil, "10.1.2.3"},
		{"trusted peer, all hops trusted -> left-most", "10.1.2.3:80", []string{"10.0.0.1, 10.0.0.2"}, "10.0.0.1"},
		{"trusted peer, garbage hop", "10.1.2.3:80", []string{"evil, 10.0.0.2"}, "10.0.0.2"},
		{"ipv6 loopback trusted", "[::1]:80", []string{"2001:db8::1"}, "2001:db8::1"},
		{"untrusted ipv6", "[2001:db8::2]:80", []string{"1.2.3.4"}, "2001:db8::2"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ClientIP(reqFrom(tc.remote, tc.xff...), trusted); got != tc.want {
				t.Fatalf("ClientIP = %q, want %q", got, tc.want)
			}
		})
	}
	// Without a trusted list XFF is never honoured.
	if got := ClientIP(reqFrom("10.1.2.3:80", "1.2.3.4"), nil); got != "10.1.2.3" {
		t.Fatalf("no trusted proxies: %q", got)
	}
}

func TestParseTrustedProxiesRejectsGarbage(t *testing.T) {
	if _, err := ParseTrustedProxies("10.0.0.0/8, nope"); err == nil {
		t.Fatal("want error")
	}
	if p, err := ParseTrustedProxies(" "); err != nil || len(p) != 0 {
		t.Fatalf("empty: %v %v", p, err)
	}
}

func TestLimiterIgnoresSpoofedXFF(t *testing.T) {
	l := NewLimiter(1.0/60.0, 2, nil)
	h := l.Limit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	codes := []int{}
	for i := 0; i < 3; i++ {
		rec := httptest.NewRecorder()
		// A fresh spoofed XFF each time must not buy a fresh bucket.
		h.ServeHTTP(rec, reqFrom("203.0.113.7:1", fmt.Sprintf("1.1.1.%d", i+1)))
		codes = append(codes, rec.Code)
	}
	if codes[0] != 204 || codes[1] != 204 || codes[2] != 429 {
		t.Fatalf("codes %v, want [204 204 429]", codes)
	}
}

func TestLimiterEvictsIdleVisitors(t *testing.T) {
	l := NewLimiter(5.0/60.0, 10, nil)
	now := time.Unix(1_000_000, 0)
	l.now = func() time.Time { return now }
	l.get("a")
	l.get("b")
	now = now.Add(l.ttl / 2)
	l.get("b") // b stays fresh
	now = now.Add(l.ttl/2 + time.Second)
	l.get("c") // triggers a sweep: a idle >= ttl, b idle < ttl
	if _, ok := l.visitors["a"]; ok {
		t.Fatal("idle visitor a not evicted")
	}
	if _, ok := l.visitors["b"]; !ok {
		t.Fatal("recent visitor b evicted")
	}
	if len(l.visitors) != 2 {
		t.Fatalf("visitors = %d, want 2", len(l.visitors))
	}
	// ttl covers a full bucket refill, so eviction never relaxes the limit.
	if refill := time.Duration(10 / (5.0 / 60.0) * float64(time.Second)); l.ttl < refill {
		t.Fatalf("ttl %v < refill %v", l.ttl, refill)
	}
}
