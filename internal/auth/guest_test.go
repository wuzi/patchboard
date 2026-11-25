package auth_test

import (
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"testing"

	"patchboard/internal/api"
	"patchboard/internal/httpx"
	"patchboard/internal/testutil"
)

func me(t *testing.T, c *testutil.Client) api.Me {
	t.Helper()
	var m api.Me
	c.Do("GET", "/api/me", nil).Want(t, 200).JSON(t, &m)
	return m
}

func ipHash(app *testutil.App, ip string) string {
	return httpx.NewIPHasher(app.Cfg.Secret).Hash(netip.MustParseAddr(ip))
}

func placeholderAdmin(t *testing.T, app *testutil.App) {
	testutil.Exec(t, app.DB, `INSERT OR IGNORE INTO admins (id, username, display_name, password_hash, created_at) VALUES (99, 'mod', 'Mod', 'x', 0)`)
}

func TestMe_Anonymous(t *testing.T) {
	app := testutil.New(t)
	c := app.Client(t)
	r := c.Do("GET", "/api/me", nil).Want(t, 200)
	if r.Header.Get("Set-Cookie") != "" {
		t.Fatal("anonymous /api/me must not set cookies")
	}
	var m api.Me
	r.JSON(t, &m)
	if m.Guest != nil || m.Admin != nil {
		t.Fatalf("me = %+v", m)
	}
}

func TestSetName_CreatesGuest(t *testing.T) {
	app := testutil.New(t)
	c := app.Client(t)
	r := c.Do("PUT", "/api/me/name", map[string]string{"name": "WuzeraProMAX"}).Want(t, 200)
	ck := r.SetCookie("pb_guest")
	if ck == nil || !ck.HttpOnly || ck.SameSite != http.SameSiteLaxMode || ck.MaxAge != 34560000 || ck.Secure || ck.Path != "/" {
		t.Fatalf("cookie = %+v", ck)
	}
	m := me(t, c)
	if m.Guest == nil || m.Guest.Name == nil || *m.Guest.Name != "WuzeraProMAX" {
		t.Fatalf("me = %+v", m.Guest)
	}
	testutil.Eq(t, len(m.Guest.ID), 16)
	if !strings.HasPrefix(m.Guest.RecoveryURL, "http://pb.test/r/") {
		t.Fatalf("recoveryUrl = %s", m.Guest.RecoveryURL)
	}
}

func TestSetName_TrimsAndValidates(t *testing.T) {
	app := testutil.New(t)
	c := app.Client(t)
	c.Do("PUT", "/api/me/name", map[string]string{"name": "   "}).Want(t, 200)
	if n := me(t, c).Guest.Name; n != nil {
		t.Fatalf("blank name stored as %q", *n)
	}
	r := c.Do("PUT", "/api/me/name", map[string]string{"name": strings.Repeat("a", 41)}).Want(t, 422)
	testutil.Eq(t, r.Field(t, "name"), "too_long")
	c.Do("PUT", "/api/me/name", map[string]string{"name": strings.Repeat("ç", 40)}).Want(t, 200)
	c.Do("PUT", "/api/me/name", map[string]string{"name": "  Wuzi  "}).Want(t, 200)
	testutil.Eq(t, *me(t, c).Guest.Name, "Wuzi")
}

func TestRecoveryLink(t *testing.T) {
	app := testutil.New(t)
	a := app.Client(t)
	a.Do("PUT", "/api/me/name", map[string]string{"name": "A"}).Want(t, 200)
	ma := me(t, a)
	u, _ := url.Parse(ma.Guest.RecoveryURL)

	b := app.Client(t)
	r := b.Do("GET", u.Path, nil).Want(t, 302)
	testutil.Eq(t, r.Header.Get("Location"), "/?recovered=1")
	if r.SetCookie("pb_guest") == nil {
		t.Fatal("recovery must set the guest cookie")
	}
	testutil.Eq(t, me(t, b).Guest.ID, ma.Guest.ID)

	c := app.Client(t)
	r = c.Do("GET", "/r/not-a-token", nil).Want(t, 302)
	testutil.Eq(t, r.Header.Get("Location"), "/?recovered=0")
	if r.SetCookie("pb_guest") != nil {
		t.Fatal("invalid token must not set a cookie")
	}
}

func TestGuestCreate_RateLimited(t *testing.T) {
	app := testutil.New(t)
	for i := 0; i < 20; i++ {
		app.Client(t).WithIP("203.0.113.1").Do("PUT", "/api/me/name", map[string]string{"name": "x"}).Want(t, 200)
	}
	r := app.Client(t).WithIP("203.0.113.1").Do("PUT", "/api/me/name", map[string]string{"name": "x"}).Want(t, 429)
	testutil.Eq(t, r.ErrCode(t), "rate_limited")
	if r.Header.Get("Retry-After") == "" {
		t.Fatal("missing Retry-After")
	}
}

func TestBannedIP_CannotCreateGuest(t *testing.T) {
	app := testutil.New(t)
	placeholderAdmin(t, app)
	testutil.Exec(t, app.DB, `INSERT INTO bans (ip_hash, created_by, created_at) VALUES (?, 99, 0)`, ipHash(app, "203.0.113.2"))
	r := app.Client(t).WithIP("203.0.113.2").Do("PUT", "/api/me/name", map[string]string{"name": "x"}).Want(t, 403)
	testutil.Eq(t, r.ErrCode(t), "banned")
}

func TestBannedGuest_CannotWrite(t *testing.T) {
	app := testutil.New(t)
	placeholderAdmin(t, app)
	c := app.Client(t)
	c.Do("PUT", "/api/me/name", map[string]string{"name": "x"}).Want(t, 200)
	testutil.Exec(t, app.DB, `INSERT INTO bans (guest_id, created_by, created_at) VALUES (?, 99, 0)`, me(t, c).Guest.ID)
	testutil.Eq(t, c.Do("PUT", "/api/me/name", map[string]string{"name": "y"}).Want(t, 403).ErrCode(t), "banned")
}
