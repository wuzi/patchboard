package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"patchboard/internal/clock"
	"patchboard/internal/config"
	"patchboard/internal/server"
	"patchboard/internal/store"
	"patchboard/internal/uploads"
	"patchboard/internal/web"
)

func serve(getenv func(string) string, stderr io.Writer) error {
	cfg, err := config.Load(getenv)
	if err != nil {
		return err
	}
	db, err := store.Open(filepath.Join(cfg.DataDir, "patchboard.db"))
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := store.Migrate(ctx, db); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	h, err := server.New(server.Deps{Cfg: cfg, DB: db, Clock: clock.Real{}, WebFS: web.Dist()})
	if err != nil {
		return err
	}
	go cleanupLoop(ctx, uploads.NewService(db, clock.Real{}, nil, nil, cfg.DataDir))
	srv := &http.Server{Addr: cfg.Addr, Handler: h, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()
	slog.Info("patchboard listening", "addr", cfg.Addr, "baseURL", cfg.BaseURL.String())
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// cleanupLoop removes orphaned uploads at startup and then hourly.
func cleanupLoop(ctx context.Context, svc *uploads.Service) {
	run := func() {
		if n, err := svc.CleanupOrphans(ctx); err != nil {
			slog.Error("upload cleanup failed", "err", err)
		} else if n > 0 {
			slog.Info("removed orphaned uploads", "count", n)
		}
	}
	run()
	t := time.NewTicker(time.Hour)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			run()
		}
	}
}
