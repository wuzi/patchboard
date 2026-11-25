package auth

import (
	"context"
	"net/http"

	"patchboard/internal/httpx"
	"patchboard/internal/store/gen"
)

// loadSession resolves an admin session token to an active admin, or nil.
func (s *Service) loadSession(ctx context.Context, token string) *Admin {
	row, err := s.q.GetSessionAdmin(ctx, gen.GetSessionAdminParams{TokenHash: hashToken(token), ExpiresAt: s.clock.Now().Unix()})
	if err != nil {
		return nil
	}
	return &Admin{ID: row.ID, Username: row.Username, DisplayName: row.DisplayName}
}

// startSession creates a session for adminID and sets the cookie.
func (s *Service) startSession(ctx context.Context, w http.ResponseWriter, q *gen.Queries, adminID int64) error {
	token := randomToken()
	now := s.clock.Now()
	if err := q.CreateSession(ctx, gen.CreateSessionParams{
		TokenHash: hashToken(token), AdminID: adminID, CreatedAt: now.Unix(), ExpiresAt: now.Add(sessionLength).Unix(),
	}); err != nil {
		return err
	}
	s.setCookie(w, adminCookie, token, adminMaxAge)
	return nil
}

// RequireAdmin rejects requests without an active admin session.
func (s *Service) RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !ActorFrom(r.Context()).IsAdmin() {
			httpx.WriteError(w, httpx.ErrUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
