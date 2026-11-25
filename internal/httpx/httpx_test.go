package httpx_test

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"

	"patchboard/internal/clock"
	"patchboard/internal/httpx"
)

func TestClientIP(t *testing.T) {
	p := netip.MustParsePrefix
	cases := []struct {
		name    string
		remote  string
		trusted []netip.Prefix
		xff     string
		want    string
	}{
		{"untrusted ignores header", "203.0.113.5:1234", nil, "1.2.3.4", "203.0.113.5"},
		{"trusted proxy uses header", "127.0.0.1:1234", []netip.Prefix{p("127.0.0.0/8")}, "203.0.113.9", "203.0.113.9"},
		{"rightmost untrusted", "127.0.0.1:1234", []netip.Prefix{p("127.0.0.0/8"), p("10.0.0.0/8")}, "1.1.1.1, 203.0.113.9, 10.0.0.2", "203.0.113.9"},
		{"garbage header", "127.0.0.1:1234", []netip.Prefix{p("127.0.0.0/8")}, "garbage", "127.0.0.1"},
		{"no header", "127.0.0.1:1234", []netip.Prefix{p("127.0.0.0/8")}, "", "127.0.0.1"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/", nil)
			r.RemoteAddr = c.remote
			if c.xff != "" {
				r.Header.Set("X-Forwarded-For", c.xff)
			}
			if got := httpx.ClientIP(r, c.trusted); got.String() != c.want {
				t.Fatalf("ClientIP = %s, want %s", got, c.want)
			}
		})
	}
}

func TestIPHasher(t *testing.T) {
	h := httpx.NewIPHasher([]byte("0123456789abcdef0123456789abcdef"))
	a := netip.MustParseAddr
	if h.Hash(a("2001:db8:1:2::1")) != h.Hash(a("2001:db8:1:2:ffff::9")) {
		t.Fatal("same /64 should hash equal")
	}
	if h.Hash(a("2001:db8:1:2::1")) == h.Hash(a("2001:db8:1:3::1")) {
		t.Fatal("different /64 should differ")
	}
	if h.Hash(a("::ffff:203.0.113.7")) != h.Hash(a("203.0.113.7")) {
		t.Fatal("v4-mapped should equal v4")
	}
	if got := len(h.Hash(a("203.0.113.7"))); got != 32 {
		t.Fatalf("len = %d", got)
	}
	other := httpx.NewIPHasher([]byte("another-secret-another-secret-xx"))
	if other.Hash(a("203.0.113.7")) == h.Hash(a("203.0.113.7")) {
		t.Fatal("different secret should differ")
	}
}

func TestLimiter(t *testing.T) {
	c := clock.NewFake(time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC))
	l := httpx.NewLimiter(c)
	rule := httpx.Rule{Name: "x", Limit: 5, Window: time.Hour}
	for i := 0; i < 5; i++ {
		if ok, _ := l.Allow(rule, "k"); !ok {
			t.Fatalf("call %d denied", i+1)
		}
	}
	ok, retry := l.Allow(rule, "k")
	if ok {
		t.Fatal("6th call allowed")
	}
	if retry <= 0 || retry > time.Hour {
		t.Fatalf("retryAfter = %v", retry)
	}
	if ok, _ := l.Allow(rule, "other"); !ok {
		t.Fatal("other key should be independent")
	}
	c.Advance(time.Hour)
	if ok, _ := l.Allow(rule, "k"); !ok {
		t.Fatal("allowed again after window")
	}
}

func TestCSRF(t *testing.T) {
	reached := false
	h := httpx.CSRF(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { reached = true }))
	do := func(method, path string, header bool) (int, bool) {
		reached = false
		r := httptest.NewRequest(method, path, nil)
		if header {
			r.Header.Set("X-PB-CSRF", "1")
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code, reached
	}
	if code, ok := do("POST", "/api/x", false); code != 403 || ok {
		t.Fatalf("POST without header = %d reached=%v", code, ok)
	}
	if _, ok := do("POST", "/api/x", true); !ok {
		t.Fatal("POST with header should pass")
	}
	if _, ok := do("GET", "/api/x", false); !ok {
		t.Fatal("GET should pass")
	}
	if _, ok := do("POST", "/r/abc", false); !ok {
		t.Fatal("non-api POST should pass")
	}
}

func TestRequestInfo(t *testing.T) {
	h := httpx.NewIPHasher([]byte("0123456789abcdef0123456789abcdef"))
	var got httpx.Info
	mw := httpx.RequestInfo([]netip.Prefix{netip.MustParsePrefix("127.0.0.0/8")}, h)
	srv := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { got = httpx.InfoFrom(r.Context()) }))
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "127.0.0.1:1"
	r.Header.Set("X-Forwarded-For", "198.51.100.4")
	srv.ServeHTTP(httptest.NewRecorder(), r)
	if got.IP.String() != "198.51.100.4" || got.IPHash != h.Hash(netip.MustParseAddr("198.51.100.4")) {
		t.Fatalf("info = %+v", got)
	}
}

func TestLimiter_ExceededDoesNotRecord(t *testing.T) {
	c := clock.NewFake(time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC))
	l := httpx.NewLimiter(c)
	rule := httpx.Rule{Name: "x", Limit: 2, Window: time.Minute}
	for i := 0; i < 5; i++ {
		if over, _ := l.Exceeded(rule, "k"); over {
			t.Fatal("peeking must not consume the limit")
		}
	}
	l.Allow(rule, "k")
	l.Allow(rule, "k")
	if over, retry := l.Exceeded(rule, "k"); !over || retry <= 0 {
		t.Fatalf("over=%v retry=%v", over, retry)
	}
}
