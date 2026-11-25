package posts_test

import (
	"strings"
	"testing"

	"patchboard/internal/api"
	"patchboard/internal/testutil"
)

func changeStatus(t *testing.T, c *testutil.Client, id int64, body map[string]any, status int) api.PostDetail {
	t.Helper()
	var p api.PostDetail
	resp := c.Do("POST", "/api/admin"+path(id)[len("/api"):]+"/status", body).Want(t, status)
	if status == 200 {
		resp.JSON(t, &p)
	}
	return p
}

func TestChangeStatus_WithResponseAndUpdate(t *testing.T) {
	app := testutil.New(t)
	p := app.Client(t).NewPost(t, "Aumentar tamanho do chat")
	admin := app.AdminClient(t, "wuzi")
	got := changeStatus(t, admin, p.ID, map[string]any{
		"statusId": 3, "response": "Vamos fazer!",
		"update": map[string]string{"title": "Chat maior planejado", "body": "Obrigado pelos votos."},
	}, 200)
	testutil.Eq(t, got.StatusID, int64(3))
	if got.Response == nil || !strings.Contains(got.Response.HTML, "Vamos fazer!") {
		t.Fatalf("response = %+v", got.Response)
	}
	testutil.Eq(t, got.Response.Author.Kind, "admin")
	testutil.Eq(t, *got.Response.Author.Name, "Wuzi")
	testutil.Eq(t, got.Response.MD, "Vamos fazer!")

	var page api.UpdatePage
	app.Client(t).Do("GET", "/api/updates", nil).Want(t, 200).JSON(t, &page)
	testutil.Eq(t, len(page.Updates), 1)
	testutil.Eq(t, page.Updates[0].Title, "Chat maior planejado")
	testutil.Eq(t, len(page.Updates[0].Posts), 1)
	testutil.Eq(t, page.Updates[0].Posts[0].ID, p.ID)
	testutil.Eq(t, page.Updates[0].Posts[0].StatusID, int64(3))

	// guests see the response but not its markdown source
	r := getPost(t, app.Client(t), p.ID).Post
	testutil.Eq(t, r.Response.MD, "")
}

func TestChangeStatus_ClearResponse(t *testing.T) {
	app := testutil.New(t)
	p := app.Client(t).NewPost(t, "Co-op")
	admin := app.AdminClient(t, "wuzi")
	changeStatus(t, admin, p.ID, map[string]any{"statusId": 2, "response": "Analisando"}, 200)
	got := changeStatus(t, admin, p.ID, map[string]any{"statusId": 3}, 200)
	if got.Response == nil || !strings.Contains(got.Response.HTML, "Analisando") {
		t.Fatal("omitting response must keep the previous one")
	}
	got = changeStatus(t, admin, p.ID, map[string]any{"statusId": 3, "response": ""}, 200)
	if got.Response != nil {
		t.Fatalf("response not cleared: %+v", got.Response)
	}
	var page api.UpdatePage
	app.Client(t).Do("GET", "/api/updates", nil).Want(t, 200).JSON(t, &page)
	testutil.Eq(t, len(page.Updates), 0)
}

func TestChangeStatus_Errors(t *testing.T) {
	app := testutil.New(t)
	p := app.Client(t).NewPost(t, "Co-op")
	admin := app.AdminClient(t, "wuzi")
	r := admin.Do("POST", "/api/admin"+path(p.ID)[len("/api"):]+"/status", map[string]any{"statusId": 99}).Want(t, 422)
	testutil.Eq(t, r.Field(t, "statusId"), "invalid")
	app.Client(t).Do("POST", "/api/admin"+path(p.ID)[len("/api"):]+"/status", map[string]any{"statusId": 2}).Want(t, 401)
	testutil.Exec(t, app.DB, "UPDATE posts SET deleted_at = 1 WHERE id = ?", p.ID)
	changeStatus(t, admin, p.ID, map[string]any{"statusId": 2}, 404)
	live := app.Client(t).NewPost(t, "Live post")
	r = admin.Do("POST", "/api/admin"+path(live.ID)[len("/api"):]+"/status", map[string]any{"statusId": 2, "response": strings.Repeat("a", 5001)}).Want(t, 422)
	testutil.Eq(t, r.Field(t, "response"), "too_long")
}
