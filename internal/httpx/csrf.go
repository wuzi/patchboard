package httpx

import (
	"net/http"
	"strings"
)

// CSRF requires "X-PB-CSRF: 1" on state-changing /api requests. Browsers
// cannot send custom headers cross-site without a CORS preflight we never allow.
func CSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
		default:
			if strings.HasPrefix(r.URL.Path, "/api/") && r.Header.Get("X-PB-CSRF") != "1" {
				WriteError(w, E(http.StatusForbidden, "csrf", "missing X-PB-CSRF header"))
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
