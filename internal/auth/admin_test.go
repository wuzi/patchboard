package auth_test

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"patchboard/internal/api"
	"patchboard/internal/auth"
	"patchboard/internal/testutil"
)

func createAdmin(t *testing.T, app *testutil.App, username, password string) int64 {
	t.Helper()
	id, err := auth.CreateAdmin(context.Background(), app.DB, app.Clock.Now(), username, "Wuzi", password)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func login(c *testutil.Client, username, password string) *testutil.Resp {
	return c.Do("POST", "/api/admin/login", map[string]string{"username": username, "password": password})
}

func TestLogin(t *testing.T) {
	app := testutil.New(t)
	createAdmin(t, app, "wuzi", "correct-horse-1")
	c := app.Client(t)
	r := login(c, "WUZI", "correct-horse-1").Want(t, 200)
	ck := r.SetCookie("pb_admin")
	if ck == nil || ck.MaxAge != 2592000 || !ck.HttpOnly {
		t.Fatalf("cookie = %+v", ck)
	}
	m := me(t, c)
	if m.Admin == nil || m.Admin.Username != "wuzi" || m.Admin.DisplayName != "Wuzi" {
		t.Fatalf("me.admin = %+v", m.Admin)
	}
}

func TestLogin_WrongPasswordAndLimit(t *testing.T) {
	app := testutil.New(t)
	createAdmin(t, app, "wuzi", "correct-horse-1")
	c := app.Client(t)
	for i := 0; i < 5; i++ {
		testutil.Eq(t, login(c, "wuzi", "nope").Want(t, 401).ErrCode(t), "invalid_credentials")
	}
	testutil.Eq(t, login(c, "wuzi", "correct-horse-1").Want(t, 429).ErrCode(t), "rate_limited")
}

func TestLogin_UnknownUser(t *testing.T) {
	app := testutil.New(t)
	testutil.Eq(t, login(app.Client(t), "ghost", "whatever-pw").Want(t, 401).ErrCode(t), "invalid_credentials")
}

func TestLogin_DisabledAdmin(t *testing.T) {
	app := testutil.New(t)
	createAdmin(t, app, "wuzi", "correct-horse-1")
	testutil.Exec(t, app.DB, `UPDATE admins SET disabled_at = 1 WHERE username = 'wuzi'`)
	testutil.Eq(t, login(app.Client(t), "wuzi", "correct-horse-1").Want(t, 401).ErrCode(t), "invalid_credentials")
}

func TestAdminRoutes_RequireSession(t *testing.T) {
	app := testutil.New(t)
	c := app.Client(t)
	testutil.Eq(t, c.Do("GET", "/api/admin/team", nil).Want(t, 401).ErrCode(t), "unauthorized")
	c.Do("PUT", "/api/me/name", map[string]string{"name": "g"}).Want(t, 200)
	c.Do("GET", "/api/admin/team", nil).Want(t, 401)
}

func TestSession_Expires(t *testing.T) {
	app := testutil.New(t)
	c := app.AdminClient(t, "wuzi")
	c.Do("GET", "/api/admin/team", nil).Want(t, 200)
	app.Clock.Advance(30*24*time.Hour + time.Second)
	c.Do("GET", "/api/admin/team", nil).Want(t, 401)
}

func TestLogout(t *testing.T) {
	app := testutil.New(t)
	c := app.AdminClient(t, "wuzi")
	c.Do("POST", "/api/admin/logout", nil).Want(t, 204)
	c.Do("GET", "/api/admin/team", nil).Want(t, 401)
}

func TestTeam_List(t *testing.T) {
	app := testutil.New(t)
	c := app.AdminClient(t, "wuzi")
	var team []api.TeamMember
	c.Do("GET", "/api/admin/team", nil).Want(t, 200).JSON(t, &team)
	if len(team) != 1 || team[0].Username != "wuzi" || team[0].DisplayName != "Wuzi" || team[0].Disabled {
		t.Fatalf("team = %+v", team)
	}
}

func TestInvite_Flow(t *testing.T) {
	app := testutil.New(t)
	admin := app.AdminClient(t, "wuzi")
	var inv api.Invite
	admin.Do("POST", "/api/admin/invites", nil).Want(t, 201).JSON(t, &inv)
	if !strings.HasPrefix(inv.URL, "http://pb.test/admin/invite/") {
		t.Fatalf("url = %s", inv.URL)
	}
	testutil.Eq(t, inv.ExpiresAt.Equal(app.Clock.Now().Add(7*24*time.Hour)), true)
	token := strings.TrimPrefix(inv.URL, "http://pb.test/admin/invite/")

	guest := app.Client(t)
	var valid struct{ Valid bool }
	guest.Do("GET", "/api/admin/invites/"+token, nil).Want(t, 200).JSON(t, &valid)
	testutil.Eq(t, valid.Valid, true)

	accept := map[string]string{"username": "Mod.One", "displayName": "Mod", "password": "long-enough-pw"}
	guest.Do("POST", "/api/admin/invites/"+token+"/accept", accept).Want(t, 201)
	testutil.Eq(t, me(t, guest).Admin.Username, "mod.one")

	other := app.Client(t)
	accept["username"] = "mod.two"
	testutil.Eq(t, other.Do("POST", "/api/admin/invites/"+token+"/accept", accept).Want(t, 410).ErrCode(t), "invite_used")

	admin.Do("POST", "/api/admin/invites", nil).Want(t, 201).JSON(t, &inv)
	token2 := strings.TrimPrefix(inv.URL, "http://pb.test/admin/invite/")
	app.Clock.Advance(7*24*time.Hour + time.Second)
	testutil.Eq(t, other.Do("POST", "/api/admin/invites/"+token2+"/accept", accept).Want(t, 410).ErrCode(t), "invite_expired")
	other.Do("GET", "/api/admin/invites/"+token2, nil).Want(t, 200).JSON(t, &valid)
	testutil.Eq(t, valid.Valid, false)
}

func TestInvite_Validation(t *testing.T) {
	app := testutil.New(t)
	admin := app.AdminClient(t, "wuzi")
	var inv api.Invite
	admin.Do("POST", "/api/admin/invites", nil).Want(t, 201).JSON(t, &inv)
	token := strings.TrimPrefix(inv.URL, "http://pb.test/admin/invite/")
	c := app.Client(t)
	r := c.Do("POST", "/api/admin/invites/"+token+"/accept", map[string]string{"username": "WUZI", "displayName": "W", "password": "long-enough-pw"}).Want(t, 409)
	testutil.Eq(t, r.ErrCode(t), "username_taken")
	r = c.Do("POST", "/api/admin/invites/"+token+"/accept", map[string]string{"username": "a b", "displayName": "", "password": "short"}).Want(t, 422)
	testutil.Eq(t, r.Field(t, "username"), "invalid")
	testutil.Eq(t, r.Field(t, "displayName"), "required")
	testutil.Eq(t, r.Field(t, "password"), "too_short")
}

func TestDisable(t *testing.T) {
	app := testutil.New(t)
	admin := app.AdminClient(t, "wuzi")
	other := app.AdminClient(t, "mod")
	var team []api.TeamMember
	admin.Do("GET", "/api/admin/team", nil).Want(t, 200).JSON(t, &team)
	ids := map[string]int64{}
	for _, m := range team {
		ids[m.Username] = m.ID
	}
	testutil.Eq(t, admin.Do("POST", "/api/admin/team/"+itoa(ids["wuzi"])+"/disable", nil).Want(t, 422).ErrCode(t), "cannot_disable_self")
	admin.Do("POST", "/api/admin/team/"+itoa(ids["mod"])+"/disable", nil).Want(t, 204)
	other.Do("GET", "/api/admin/team", nil).Want(t, 401)
	login(app.Client(t), "mod", "correct-horse-1").Want(t, 401)
}

func TestUpdateAdminMe(t *testing.T) {
	app := testutil.New(t)
	c := app.AdminClient(t, "wuzi")
	c.Do("PATCH", "/api/admin/me", map[string]string{"displayName": "Wuzi Dev"}).Want(t, 200)
	testutil.Eq(t, me(t, c).Admin.DisplayName, "Wuzi Dev")
	r := c.Do("PATCH", "/api/admin/me", map[string]string{"currentPassword": "wrong", "newPassword": "long-enough-pw"}).Want(t, 422)
	testutil.Eq(t, r.Field(t, "currentPassword"), "invalid")
	r = c.Do("PATCH", "/api/admin/me", map[string]string{"currentPassword": "correct-horse-1", "newPassword": "short"}).Want(t, 422)
	testutil.Eq(t, r.Field(t, "newPassword"), "too_short")
	c.Do("PATCH", "/api/admin/me", map[string]string{"currentPassword": "correct-horse-1", "newPassword": "long-enough-pw"}).Want(t, 200)
	login(app.Client(t), "wuzi", "long-enough-pw").Want(t, 200)
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

func TestLogin_SuccessDoesNotCountTowardLimit(t *testing.T) {
	app := testutil.New(t)
	createAdmin(t, app, "wuzi", "correct-horse-1")
	for i := 0; i < 12; i++ {
		login(app.Client(t).WithIP("203.0.113.90"), "wuzi", "correct-horse-1").Want(t, 200)
	}
}

func TestLogin_ConcurrentAttemptsAreLimited(t *testing.T) {
	app := testutil.New(t)
	createAdmin(t, app, "wuzi", "correct-horse-1")
	const n = 40
	codes := make(chan int, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			codes <- login(app.Client(t).WithIP("203.0.113.91"), "wuzi", "wrong-password").Code
		}()
	}
	wg.Wait()
	close(codes)
	unauthorized := 0
	for c := range codes {
		if c == 401 {
			unauthorized++
		}
	}
	if unauthorized > 5 {
		t.Fatalf("%d of %d concurrent wrong logins were evaluated; the limit is 5", unauthorized, n)
	}
}

func TestUpdateAdminMe_PasswordChangeSignsOutOtherSessions(t *testing.T) {
	app := testutil.New(t)
	a := app.AdminClient(t, "wuzi")
	b := app.Client(t)
	login(b, "wuzi", testutil.AdminPassword).Want(t, 200)
	a.Do("PATCH", "/api/admin/me", map[string]string{"currentPassword": testutil.AdminPassword, "newPassword": "brand-new-pass"}).Want(t, 200)
	a.Do("GET", "/api/admin/team", nil).Want(t, 200)
	b.Do("GET", "/api/admin/team", nil).Want(t, 401)
}
