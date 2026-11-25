package httpx

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// ClientIP returns the caller's IP. X-Forwarded-For is only trusted when the
// direct peer is a trusted proxy; then the rightmost untrusted hop wins.
func ClientIP(r *http.Request, trusted []netip.Prefix) netip.Addr {
	remote := parseHostAddr(r.RemoteAddr)
	if !isTrusted(remote, trusted) {
		return remote
	}
	hops := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
	for i := len(hops) - 1; i >= 0; i-- {
		a, err := netip.ParseAddr(strings.TrimSpace(hops[i]))
		if err != nil {
			return remote
		}
		a = a.Unmap()
		if !isTrusted(a, trusted) {
			return a
		}
	}
	return remote
}

func parseHostAddr(hostport string) netip.Addr {
	host, _, err := net.SplitHostPort(hostport)
	if err != nil {
		host = hostport
	}
	a, err := netip.ParseAddr(host)
	if err != nil {
		return netip.IPv4Unspecified()
	}
	return a.Unmap()
}

func isTrusted(a netip.Addr, trusted []netip.Prefix) bool {
	for _, p := range trusted {
		if p.Contains(a) {
			return true
		}
	}
	return false
}

// IPHasher turns IPs into keyed, non-reversible identifiers.
type IPHasher struct{ secret []byte }

func NewIPHasher(secret []byte) IPHasher { return IPHasher{secret: secret} }

// Hash returns hex(HMAC-SHA256(secret, ip))[:32]; IPv6 is grouped by /64.
func (h IPHasher) Hash(a netip.Addr) string {
	a = a.Unmap()
	if a.Is6() {
		p, _ := a.Prefix(64)
		a = p.Addr()
	}
	m := hmac.New(sha256.New, h.secret)
	b, _ := a.MarshalBinary()
	m.Write(b)
	return hex.EncodeToString(m.Sum(nil))[:32]
}

// Info is per-request client information.
type Info struct {
	IP     netip.Addr
	IPHash string
}

type infoKey struct{}

// RequestInfo stores the client IP and its hash in the request context.
func RequestInfo(trusted []netip.Prefix, h IPHasher) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := ClientIP(r, trusted)
			ctx := context.WithValue(r.Context(), infoKey{}, Info{IP: ip, IPHash: h.Hash(ip)})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// InfoFrom returns the Info stored by RequestInfo (zero value if absent).
func InfoFrom(ctx context.Context) Info {
	i, _ := ctx.Value(infoKey{}).(Info)
	return i
}
