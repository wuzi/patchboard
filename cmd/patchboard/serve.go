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
	"testing/fstest"
	"time"

	"patchboard/internal/clock"
	"patchboard/internal/config"
	"patchboard/internal/server"
	"patchboard/internal/store"
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
	h, err := server.New(server.Deps{Cfg: cfg, DB: db, Clock: clock.Real{}, WebFS: fstest.MapFS{}})
	if err != nil {
		return err
	}
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
