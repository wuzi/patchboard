package posts_test

import (
	"fmt"
	"testing"
	"time"

	"patchboard/internal/api"
	"patchboard/internal/testutil"
)

func adminPath(id int64, action string) string {
	return fmt.Sprintf("/api/admin/posts/%d/%s", id, action)
}

func TestApproval(t *testing.T) {
	app := testutil.New(t)
	enableApproval(t, app)
	author := app.Client(t)
	first := author.NewPost(t, "Primeiro pendente")
	app.Clock.Advance(time.Second)
	second := author.NewPost(t, "Segundo pendente")
	admin := app.AdminClient(t, "wuzi")
	var pending []api.PostCard
	admin.Do("GET", "/api/admin/pending", nil).Want(t, 200).JSON(t, &pending)
	eqStrings(t, titles(pending), "Primeiro pendente", "Segundo pendente")
	app.Client(t).Do("GET", "/api/admin/pending", nil).Want(t, 401)

	var p api.PostDetail
	admin.Do("POST", adminPath(first.ID, "approve"), nil).Want(t, 200).JSON(t, &p)
	testutil.Eq(t, p.Pending, false)
	eqStrings(t, titles(list(t, app.Client(t), "").Posts), "Primeiro pendente")
	admin.Do("GET", "/api/admin/pending", nil).Want(t, 200).JSON(t, &pending)
	eqStrings(t, titles(pending), "Segundo pendente")
	_ = second
}

func TestPinAndLock(t *testing.T) {
	app := testutil.New(t)
	author := app.AdminClient(t, "wuzi")
	a := author.NewPost(t, "Post A")
	b := author.NewPost(t, "Post B")
	app.Client(t).Vote(t, b.ID)
	var p api.PostDetail
	author.Do("POST", adminPath(a.ID, "pin"), map[string]bool{"pinned": true}).Want(t, 200).JSON(t, &p)
	testutil.Eq(t, p.Pinned, true)
	eqStrings(t, titles(list(t, app.Client(t), "").Posts), "Post A", "Post B")
	author.Do("POST", adminPath(a.ID, "pin"), map[string]bool{"pinned": false}).Want(t, 200).JSON(t, &p)
	testutil.Eq(t, p.Pinned, false)
	eqStrings(t, titles(list(t, app.Client(t), "").Posts), "Post B", "Post A")

	author.Do("POST", adminPath(a.ID, "lock"), map[string]bool{"locked": true}).Want(t, 200).JSON(t, &p)
	testutil.Eq(t, p.CommentsLocked, true)
	guest := app.Client(t)
	testutil.Eq(t, guest.Do("POST", fmt.Sprintf("/api/posts/%d/comments", a.ID), map[string]string{"body": "oi"}).Want(t, 403).ErrCode(t), "locked")
	author.Do("POST", adminPath(a.ID, "lock"), map[string]bool{"locked": false}).Want(t, 200)
	guest.Do("POST", fmt.Sprintf("/api/posts/%d/comments", a.ID), map[string]string{"body": "oi"}).Want(t, 201)
	author.Do("POST", adminPath(999, "pin"), map[string]bool{"pinned": true}).Want(t, 404)
}

func TestRestore(t *testing.T) {
	app := testutil.New(t)
	author := app.Client(t)
	p := author.NewPost(t, "Apagado pelo autor")
	var c api.Comment
	author.Do("POST", fmt.Sprintf("/api/posts/%d/comments", p.ID), map[string]string{"body": "comentário"}).Want(t, 201).JSON(t, &c)
	author.Do("DELETE", fmt.Sprintf("/api/comments/%d", c.ID), nil).Want(t, 204)
	author.Do("DELETE", path(p.ID), nil).Want(t, 204)
	admin := app.AdminClient(t, "wuzi")
	q := author.NewPost(t, "Apagado pelo admin")
	admin.Do("DELETE", path(q.ID), nil).Want(t, 204)

	var trash api.Trash
	admin.Do("GET", "/api/admin/trash", nil).Want(t, 200).JSON(t, &trash)
	testutil.Eq(t, len(trash.Posts), 2)
	by := map[string]string{}
	for _, tp := range trash.Posts {
		by[tp.Title] = tp.DeletedBy
	}
	testutil.Eq(t, by["Apagado pelo autor"], "author")
	testutil.Eq(t, by["Apagado pelo admin"], "admin")
	testutil.Eq(t, len(trash.Comments), 1)
	testutil.Eq(t, trash.Comments[0].PostTitle, "Apagado pelo autor")
	testutil.Eq(t, trash.Comments[0].Excerpt, "comentário")
	testutil.Eq(t, trash.Comments[0].DeletedBy, "author")

	admin.Do("POST", adminPath(p.ID, "restore"), nil).Want(t, 204)
	admin.Do("POST", fmt.Sprintf("/api/admin/comments/%d/restore", c.ID), nil).Want(t, 204)
	got := getPost(t, app.Client(t), p.ID).Post
	testutil.Eq(t, got.CommentCount, int64(1))
	testutil.Eq(t, len(got.Comments), 1)
	admin.Do("GET", "/api/admin/trash", nil).Want(t, 200).JSON(t, &trash)
	testutil.Eq(t, len(trash.Posts), 1)
	testutil.Eq(t, len(trash.Comments), 0)
	app.Client(t).Do("GET", "/api/admin/trash", nil).Want(t, 401)
}
