package updates_test

import (
	"fmt"
	"testing"
	"time"

	"patchboard/internal/api"
	"patchboard/internal/testutil"
)

func create(t *testing.T, c *testutil.Client, body map[string]any, status int) api.UpdateItem {
	t.Helper()
	var u api.UpdateItem
	r := c.Do("POST", "/api/admin/updates", body).Want(t, status)
	if status == 201 {
		r.JSON(t, &u)
	}
	return u
}

func list(t *testing.T, c *testutil.Client, q string) api.UpdatePage {
	t.Helper()
	var p api.UpdatePage
	c.Do("GET", "/api/updates"+q, nil).Want(t, 200).JSON(t, &p)
	return p
}

func TestUpdate_StatusSnapshot(t *testing.T) {
	app := testutil.New(t)
	p := app.Client(t).NewPost(t, "Dark mode")
	admin := app.AdminClient(t, "wuzi")
	testutil.Exec(t, app.DB, "UPDATE posts SET status_id = 3 WHERE id = ?", p.ID)
	u := create(t, admin, map[string]any{"title": "Dark mode is planned", "body": "Thanks for the votes.", "postIds": []int64{p.ID}}, 201)
	testutil.Eq(t, u.Posts[0].StatusID, int64(3))
	testutil.Eq(t, u.Author.Kind, "admin")
	testutil.Eq(t, u.BodyMD, "Thanks for the votes.")
	admin.Do("POST", fmt.Sprintf("/api/admin/posts/%d/status", p.ID), map[string]any{"statusId": 5}).Want(t, 200)
	got := list(t, app.Client(t), "")
	testutil.Eq(t, got.Updates[0].Posts[0].StatusID, int64(3))
	testutil.Eq(t, got.Updates[0].BodyMD, "")
}

func TestUpdates_Validation(t *testing.T) {
	app := testutil.New(t)
	admin := app.AdminClient(t, "wuzi")
	r := admin.Do("POST", "/api/admin/updates", map[string]any{"title": " ", "body": "x", "postIds": []int64{}}).Want(t, 422)
	testutil.Eq(t, r.Field(t, "title"), "required")
	ids := make([]int64, 11)
	for i := range ids {
		ids[i] = int64(i + 1)
	}
	testutil.Eq(t, admin.Do("POST", "/api/admin/updates", map[string]any{"title": "t", "body": "b", "postIds": ids}).Want(t, 422).Field(t, "postIds"), "too_many")
	testutil.Eq(t, admin.Do("POST", "/api/admin/updates", map[string]any{"title": "t", "body": "b", "postIds": []int64{999}}).Want(t, 422).Field(t, "postIds"), "invalid")
	testutil.Eq(t, admin.Do("POST", "/api/admin/updates", map[string]any{"title": "t", "body": ""}).Want(t, 422).Field(t, "body"), "required")
	app.Client(t).Do("POST", "/api/admin/updates", map[string]any{"title": "t", "body": "b"}).Want(t, 401)
}

func TestUpdates_HideDeletedPosts(t *testing.T) {
	app := testutil.New(t)
	author := app.Client(t)
	shown, deleted, pending, merged := author.NewPost(t, "Shown"), author.NewPost(t, "Deleted"), author.NewPost(t, "Pending"), author.NewPost(t, "Merged")
	admin := app.AdminClient(t, "wuzi")
	create(t, admin, map[string]any{"title": "Weekly", "body": "news", "postIds": []int64{shown.ID, deleted.ID, pending.ID, merged.ID}}, 201)
	testutil.Exec(t, app.DB, "UPDATE posts SET deleted_at = 1 WHERE id = ?", deleted.ID)
	testutil.Exec(t, app.DB, "UPDATE posts SET pending = 1 WHERE id = ?", pending.ID)
	testutil.Exec(t, app.DB, "UPDATE posts SET merged_into_id = ? WHERE id = ?", shown.ID, merged.ID)
	got := list(t, app.Client(t), "")
	testutil.Eq(t, len(got.Updates[0].Posts), 1)
	testutil.Eq(t, got.Updates[0].Posts[0].Title, "Shown")
}

func TestUpdates_EditDeletePaginate(t *testing.T) {
	app := testutil.New(t)
	a := app.Client(t).NewPost(t, "Post A")
	b := app.Client(t).NewPost(t, "Post B")
	admin := app.AdminClient(t, "wuzi")
	u := create(t, admin, map[string]any{"title": "Old title", "body": "b", "postIds": []int64{a.ID}}, 201)
	testutil.Exec(t, app.DB, "UPDATE posts SET status_id = 4 WHERE id IN (?, ?)", a.ID, b.ID)
	var e api.UpdateItem
	admin.Do("PATCH", fmt.Sprintf("/api/admin/updates/%d", u.ID), map[string]any{"title": "New title", "postIds": []int64{b.ID, a.ID}}).Want(t, 200).JSON(t, &e)
	testutil.Eq(t, e.Title, "New title")
	testutil.Eq(t, e.Posts[0].ID, b.ID)
	testutil.Eq(t, e.Posts[0].StatusID, int64(4)) // new link snapshots the current status
	testutil.Eq(t, e.Posts[1].StatusID, int64(1)) // kept link keeps its original snapshot
	admin.Do("DELETE", fmt.Sprintf("/api/admin/updates/%d", u.ID), nil).Want(t, 204)
	testutil.Eq(t, len(list(t, app.Client(t), "").Updates), 0)
	admin.Do("PATCH", fmt.Sprintf("/api/admin/updates/%d", u.ID), map[string]any{"title": "x"}).Want(t, 404)

	for i := 0; i < 25; i++ {
		create(t, admin, map[string]any{"title": fmt.Sprintf("Update %d", i), "body": "b"}, 201)
		app.Clock.Advance(time.Minute)
	}
	page := list(t, app.Client(t), "")
	testutil.Eq(t, len(page.Updates), 20)
	testutil.Eq(t, page.Updates[0].Title, "Update 24")
	testutil.Eq(t, *page.NextOffset, 20)
	page = list(t, app.Client(t), "?offset=20")
	testutil.Eq(t, len(page.Updates), 5)
	if page.NextOffset != nil {
		t.Fatal("last page must have no nextOffset")
	}
}
