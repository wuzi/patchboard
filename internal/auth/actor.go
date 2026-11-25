// Package auth handles guest identities, admin accounts and bans.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base32"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"time"

	"patchboard/internal/clock"
	"patchboard/internal/config"
	"patchboard/internal/httpx"
	"patchboard/internal/store/gen"
)

const (
	guestCookie   = "pb_guest"
	adminCookie   = "pb_admin"
	guestMaxAge   = 34560000 // 400 days, the browser cap
	adminMaxAge   = 2592000  // 30 days
	sessionLength = 30 * 24 * time.Hour
)

type Guest struct {
	ID        string
	Name      *string
	CreatedAt time.Time
}

type Admin struct {
	ID          int64
	Username    string
	DisplayName string
}

// Actor is who is making the request. Guest and Admin may both be set
// (an admin with a guest cookie votes as that guest).
type Actor struct {
	Guest *Guest
	Admin *Admin
	Info  httpx.Info

	guestToken string
}

func (a *Actor) IsAdmin() bool { return a != nil && a.Admin != nil }

// AuthorRef identifies who authored a post, comment or upload: exactly one field is set.
type AuthorRef struct {
	GuestID *string
	AdminID *int64
}

type actorKey struct{}

// ActorFrom returns the request's actor; never nil behind Middleware.
func ActorFrom(ctx context.Context) *Actor {
	if a, ok := ctx.Value(actorKey{}).(*Actor); ok {
		return a
	}
	return &Actor{Info: httpx.InfoFrom(ctx)}
}

type Service struct {
	db    *sql.DB
	q     *gen.Queries
	clock clock.Clock
	lim   *httpx.Limiter
	cfg   config.Config
}

func NewService(db *sql.DB, c clock.Clock, lim *httpx.Limiter, cfg config.Config) *Service {
	return &Service{db: db, q: gen.New(db), clock: c, lim: lim, cfg: cfg}
}

// Middleware resolves the guest and admin session cookies into an Actor.
func (s *Service) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		a := &Actor{Info: httpx.InfoFrom(r.Context())}
		if ck, err := r.Cookie(guestCookie); err == nil && ck.Value != "" {
			if g, err := s.q.GetGuestByTokenHash(r.Context(), hashToken(ck.Value)); err == nil {
				a.Guest = guestFromRow(g)
				a.guestToken = ck.Value
			}
		}
		if ck, err := r.Cookie(adminCookie); err == nil && ck.Value != "" {
			a.Admin = s.loadSession(r.Context(), ck.Value)
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), actorKey{}, a)))
	})
}

func guestFromRow(g gen.Guest) *Guest {
	out := &Guest{ID: g.ID, CreatedAt: time.Unix(g.CreatedAt, 0).UTC()}
	if g.Name.Valid {
		n := g.Name.String
		out.Name = &n
	}
	return out
}

// EnsureGuest returns the request's guest, creating one (and its cookie) if needed.
func (s *Service) EnsureGuest(w http.ResponseWriter, r *http.Request) (*Guest, error) {
	a := ActorFrom(r.Context())
	ctx := r.Context()
	if a.Guest != nil {
		if err := s.q.TouchGuestIP(ctx, gen.TouchGuestIPParams{LastIpHash: a.Info.IPHash, ID: a.Guest.ID}); err != nil {
			return nil, err
		}
		s.setCookie(w, guestCookie, a.guestToken, guestMaxAge)
		return a.Guest, nil
	}
	if err := s.CheckNotBanned(ctx, "", a.Info.IPHash); err != nil {
		return nil, err
	}
	if !a.IsAdmin() {
		if ok, retry := s.lim.Allow(httpx.RuleGuestCreate, a.Info.IPHash); !ok {
			return nil, httpx.RateLimited(retry)
		}
	}
	token := randomToken()
	id := randomID(10)
	now := s.clock.Now()
	if err := s.q.CreateGuest(ctx, gen.CreateGuestParams{
		ID: id, TokenHash: hashToken(token), CreatedAt: now.Unix(),
		CreatedIpHash: a.Info.IPHash, LastIpHash: a.Info.IPHash,
	}); err != nil {
		return nil, err
	}
	a.Guest = &Guest{ID: id, CreatedAt: now}
	a.guestToken = token
	s.setCookie(w, guestCookie, token, guestMaxAge)
	return a.Guest, nil
}

// Writer returns the author for a post/comment/upload: admins act as admins,
// everyone else as a (possibly new) guest that must not be banned.
func (s *Service) Writer(w http.ResponseWriter, r *http.Request) (AuthorRef, error) {
	a := ActorFrom(r.Context())
	if a.IsAdmin() {
		id := a.Admin.ID
		return AuthorRef{AdminID: &id}, nil
	}
	g, err := s.GuestWriter(w, r)
	if err != nil {
		return AuthorRef{}, err
	}
	id := g.ID
	return AuthorRef{GuestID: &id}, nil
}

// GuestWriter always acts as a guest identity (votes, likes, names), admins included.
func (s *Service) GuestWriter(w http.ResponseWriter, r *http.Request) (*Guest, error) {
	g, err := s.EnsureGuest(w, r)
	if err != nil {
		return nil, err
	}
	a := ActorFrom(r.Context())
	if !a.IsAdmin() {
		if err := s.CheckNotBanned(r.Context(), g.ID, a.Info.IPHash); err != nil {
			return nil, err
		}
	}
	return g, nil
}

// CheckNotBanned returns 403 banned when a ban matches the guest id or the IP hash.
func (s *Service) CheckNotBanned(ctx context.Context, guestID, ipHash string) error {
	n, err := s.q.CountBansFor(ctx, gen.CountBansForParams{
		GuestID: sql.NullString{String: guestID, Valid: guestID != ""},
		IpHash:  sql.NullString{String: ipHash, Valid: ipHash != ""},
	})
	if err != nil {
		return err
	}
	if n > 0 {
		return httpx.E(http.StatusForbidden, "banned", "you have been banned from this board")
	}
	return nil
}

// Limit applies a rate rule keyed by IP hash; admins are exempt.
func (s *Service) Limit(r *http.Request, rule httpx.Rule) error {
	a := ActorFrom(r.Context())
	if a.IsAdmin() {
		return nil
	}
	if ok, retry := s.lim.Allow(rule, a.Info.IPHash); !ok {
		return httpx.RateLimited(retry)
	}
	return nil
}

func (s *Service) setCookie(w http.ResponseWriter, name, value string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name: name, Value: value, Path: "/", MaxAge: maxAge,
		HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: s.cfg.SecureCookies,
	})
}

func (s *Service) clearCookie(w http.ResponseWriter, name string) {
	http.SetCookie(w, &http.Cookie{
		Name: name, Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: s.cfg.SecureCookies,
	})
}

func randomToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

var b32 = base32.StdEncoding.WithPadding(base32.NoPadding)

// randomID returns n random bytes as lowercase base32 (n=10 → 16 chars).
func randomID(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return strings.ToLower(b32.EncodeToString(b))
}

func hashToken(token string) []byte {
	h := sha256.Sum256([]byte(token))
	return h[:]
}

var errNoRows = sql.ErrNoRows

func isNoRows(err error) bool { return errors.Is(err, errNoRows) }
