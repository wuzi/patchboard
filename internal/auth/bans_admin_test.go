package auth_test

import (
	"fmt"
	"testing"

	"patchboard/internal/api"
	"patchboard/internal/testutil"
)

func guestOf(t *testing.T, c *testutil.Client, name string) string {
	t.Helper()
	c.Do("PUT", "/api/me/name", map[string]string{"name": name}).Want(t, 200)
	return me(t, c).Guest.ID
}

func TestBan_GuestOnly(t *testing.T) {
	app := testutil.New(t)
	admin := app.AdminClient(t, "wuzi")
	p := admin.NewPost(t, "Co-op")
	g1 := app.Client(t).WithIP("203.0.113.70")
	id := guestOf(t, g1, "Spammer")
	var ban api.Ban
	admin.Do("POST", "/api/admin/bans", map[string]any{"guestId": id, "includeIp": false, "reason": "spam"}).Want(t, 201).JSON(t, &ban)
	testutil.Eq(t, *ban.GuestID, id)
	if ban.IPHash != nil {
		t.Fatalf("ip hash stored without includeIp: %v", *ban.IPHash)
	}
	testutil.Eq(t, g1.Do("POST", fmt.Sprintf("/api/posts/%d/vote", p.ID), nil).Want(t, 403).ErrCode(t), "banned")
	app.Client(t).WithIP("203.0.113.70").Do("POST", fmt.Sprintf("/api/posts/%d/vote", p.ID), nil).Want(t, 200)
}

func TestBan_WithIP(t *testing.T) {
	app := testutil.New(t)
	admin := app.AdminClient(t, "wuzi")
	g1 := app.Client(t).WithIP("203.0.113.71")
	id := guestOf(t, g1, "Spammer")
	var ban api.Ban
	admin.Do("POST", "/api/admin/bans", map[string]any{"guestId": id, "includeIp": true, "reason": ""}).Want(t, 201).JSON(t, &ban)
	testutil.Eq(t, len(*ban.IPHash), 8)
	fresh := app.Client(t).WithIP("203.0.113.71")
	testutil.Eq(t, fresh.Do("PUT", "/api/me/name", map[string]string{"name": "alt"}).Want(t, 403).ErrCode(t), "banned")
	admin.Do("DELETE", fmt.Sprintf("/api/admin/bans/%d", ban.ID), nil).Want(t, 204)
	fresh.Do("PUT", "/api/me/name", map[string]string{"name": "alt"}).Want(t, 200)
	admin.Do("DELETE", fmt.Sprintf("/api/admin/bans/%d", ban.ID), nil).Want(t, 404)
}

func TestBans_List(t *testing.T) {
	app := testutil.New(t)
	admin := app.AdminClient(t, "wuzi")
	id := guestOf(t, app.Client(t), "Spammer")
	admin.Do("POST", "/api/admin/bans", map[string]any{"guestId": id, "includeIp": true, "reason": "flood"}).Want(t, 201)
	var bans []api.Ban
	admin.Do("GET", "/api/admin/bans", nil).Want(t, 200).JSON(t, &bans)
	testutil.Eq(t, len(bans), 1)
	testutil.Eq(t, *bans[0].GuestName, "Spammer")
	testutil.Eq(t, bans[0].Reason, "flood")
	testutil.Eq(t, admin.Do("POST", "/api/admin/bans", map[string]any{"guestId": "nope", "includeIp": false}).Want(t, 422).Field(t, "guestId"), "invalid")
	app.Client(t).Do("GET", "/api/admin/bans", nil).Want(t, 401)
}
