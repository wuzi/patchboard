// Package server wires every domain package into one http.Handler.
package server

import (
	"context"
	"database/sql"
	"io/fs"
	"net/http"

	"patchboard/internal/api"
	"patchboard/internal/auth"
	"patchboard/internal/board"
	"patchboard/internal/clock"
	"patchboard/internal/comments"
	"patchboard/internal/config"
	"patchboard/internal/httpx"
	"patchboard/internal/markdown"
	"patchboard/internal/posts"
	"patchboard/internal/updates"
	"patchboard/internal/uploads"
	"patchboard/internal/web"
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
	authSvc.RegisterBans(mux)
	boardSvc := board.NewService(d.DB)
	boardSvc.Register(mux, authSvc.RequireAdmin)
	uploadsSvc := uploads.NewService(d.DB, d.Clock, authSvc, boardSvc, d.Cfg.DataDir)
	uploadsSvc.Register(mux, authSvc.RequireAdmin)
	updatesSvc := updates.NewService(d.DB, d.Clock)
	updatesSvc.Register(mux, authSvc.RequireAdmin)
	var commentsSvc *comments.Service
	postsSvc := posts.NewService(posts.Deps{
		DB: d.DB, Clock: d.Clock, Auth: authSvc, Board: boardSvc,
		Comments: func(ctx context.Context, postID int64, viewer *auth.Actor) ([]api.Comment, error) {
			return commentsSvc.ForPost(ctx, postID, viewer)
		},
		Images:       uploadsSvc,
		CreateUpdate: updatesSvc.CreateTx,
	})
	postsSvc.Register(mux, authSvc.RequireAdmin)
	commentsSvc = comments.NewService(d.DB, d.Clock, authSvc, postsSvc)
	commentsSvc.Register(mux, authSvc.RequireAdmin)
	mux.Handle("GET /api/admin/trash", authSvc.RequireAdmin(trashHandler(postsSvc, commentsSvc)))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := d.DB.PingContext(r.Context()); err != nil {
			http.Error(w, "db unavailable", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok"))
	})
	mux.Handle("POST /api/markdown/preview", markdown.PreviewHandler(d.Limiter))
	mux.Handle("/", web.Handler(d.WebFS, metaSource{cfg: d.Cfg, board: boardSvc, posts: postsSvc}))
	mux.Handle("/api/", httpx.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
		return httpx.ErrNotFound
	}))
	var h http.Handler = mux
	h = authSvc.Middleware(h)
	h = httpx.CSRF(h)
	h = httpx.RequestInfo(d.Cfg.TrustedProxies, hasher)(h)
	return h, nil
}
