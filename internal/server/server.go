// Package server wires every domain package into one http.Handler.
package server

import (
	"database/sql"
	"io/fs"
	"net/http"

	"patchboard/internal/clock"
	"patchboard/internal/config"
	"patchboard/internal/httpx"
)

type Deps struct {
	Cfg   config.Config
	DB    *sql.DB
	Clock clock.Clock
	WebFS fs.FS
}

// New builds the full route table.
func New(d Deps) (http.Handler, error) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := d.DB.PingContext(r.Context()); err != nil {
			http.Error(w, "db unavailable", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok"))
	})
	mux.Handle("/api/", httpx.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
		return httpx.ErrNotFound
	}))
	return mux, nil
}
