package testutil_test

import (
	"testing"

	"patchboard/internal/testutil"
)

func TestHarness(t *testing.T) {
	app := testutil.New(t)
	a, b := app.Client(t), app.Client(t)
	a.Do("GET", "/healthz", nil).Want(t, 200)
	if a.IP == b.IP {
		t.Fatalf("clients share IP %s", a.IP)
	}
	// CSRF header is sent by default, so an unknown API route is a 404, not a 403.
	if code := a.Do("POST", "/api/nope", map[string]any{}).Want(t, 404).ErrCode(t); code != "not_found" {
		t.Fatalf("code = %s", code)
	}
	if code := a.NoCSRF().Do("POST", "/api/nope", map[string]any{}).Want(t, 403).ErrCode(t); code != "csrf" {
		t.Fatalf("code = %s", code)
	}
	testutil.Eq(t, app.Clock.Now().Format("2006-01-02T15:04:05Z"), "2026-11-25T12:00:00Z")
}
