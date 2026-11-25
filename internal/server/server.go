// Package server wires every domain package into one http.Handler.
package server

import (
	"database/sql"
	"io/fs"
	"net/http"

	"patchboard/internal/auth"
	"patchboard/internal/board"
	"patchboard/internal/clock"
	"patchboard/internal/config"
	"patchboard/internal/httpx"
	"patchboard/internal/markdown"
	"patchboard/internal/posts"
)

type Deps struct {
	Cfg   config.Config
	DB    *sql.DB
	Clock clock.Clock
	WebFS fs.FS
	// Limiter is shared by all handlers; nil means New creates one.
	Limiter *httpx.Limiter
}

// New builds the full route table.
func New(d Deps) (http.Handler, error) {
	if d.Limiter == nil {
		d.Limiter = httpx.NewLimiter(d.Clock)
	}
	hasher := httpx.NewIPHasher(d.Cfg.Secret)
	authSvc := auth.NewService(d.DB, d.Clock, d.Limiter, d.Cfg)
	mux := http.NewServeMux()
	authSvc.Register(mux)
	authSvc.RegisterAdmin(mux)
	boardSvc := board.NewService(d.DB)
	boardSvc.Register(mux, authSvc.RequireAdmin)
	postsSvc := posts.NewService(posts.Deps{
		DB: d.DB, Clock: d.Clock, Auth: authSvc, Board: boardSvc,
		Comments: noComments, Images: noImages{},
	})
	postsSvc.Register(mux, authSvc.RequireAdmin)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := d.DB.PingContext(r.Context()); err != nil {
			http.Error(w, "db unavailable", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok"))
	})
	mux.Handle("POST /api/markdown/preview", markdown.PreviewHandler(d.Limiter))
	mux.Handle("/api/", httpx.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
		return httpx.ErrNotFound
	}))
	var h http.Handler = mux
	h = authSvc.Middleware(h)
	h = httpx.CSRF(h)
	h = httpx.RequestInfo(d.Cfg.TrustedProxies, hasher)(h)
	return h, nil
}
