package server_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"
	"testing/fstest"
	"time"

	"patchboard/internal/clock"
	"patchboard/internal/config"
	"patchboard/internal/server"
	"patchboard/internal/store"
)

func newServer(t *testing.T) *httptest.Server {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "pb.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := store.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	base, _ := url.Parse("http://pb.test")
	h, err := server.New(server.Deps{
		Cfg:   config.Config{BaseURL: base, Secret: make([]byte, 32), DataDir: t.TempDir()},
		DB:    db,
		Clock: clock.NewFake(time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)),
		WebFS: fstest.MapFS{"index.html": {Data: []byte("<html><head><!-- pb:meta --></head></html>")}},
	})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv
}

func TestHealthz(t *testing.T) {
	srv := newServer(t)
	res, err := http.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != 200 || string(body) != "ok" {
		t.Fatalf("healthz = %d %q", res.StatusCode, body)
	}
}

func TestUnknownAPIRouteIsJSON404(t *testing.T) {
	srv := newServer(t)
	res, err := http.Get(srv.URL + "/api/nope")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var body struct {
		Error struct{ Code string } `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if res.StatusCode != 404 || body.Error.Code != "not_found" {
		t.Fatalf("got %d %q", res.StatusCode, body.Error.Code)
	}
}
