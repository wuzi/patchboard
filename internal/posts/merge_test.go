package posts_test

import (
	"fmt"
	"net/url"
	"testing"

	"patchboard/internal/api"
	"patchboard/internal/testutil"
)

func merge(t *testing.T, c *testutil.Client, from, into int64, status int) api.PostDetail {
	t.Helper()
	var p api.PostDetail
	r := c.Do("POST", adminPath(from, "merge"), map[string]int64{"intoId": into}).Want(t, status)
	if status == 200 {
		r.JSON(t, &p)
	}
	return p
}

func TestMerge(t *testing.T) {
	app := testutil.New(t)
	author := app.Client(t)
	a := author.NewPost(t, "Assassin dual daggers")
	b := author.NewPost(t, "Adagas duplas para assassin")
	g1, g2, g3 := app.Client(t), app.Client(t), app.Client(t)
	g1.Vote(t, a.ID)
	g2.Vote(t, a.ID)
	g2.Vote(t, b.ID)
	g3.Vote(t, b.ID)
	author.Do("POST", fmt.Sprintf("/api/posts/%d/comments", a.ID), map[string]string{"body": "quero muito"}).Want(t, 201)

	admin := app.AdminClient(t, "wuzi")
	target := merge(t, admin, a.ID, b.ID, 200)
	testutil.Eq(t, target.ID, b.ID)
	testutil.Eq(t, target.VoteCount, int64(3))
	testutil.Eq(t, target.CommentCount, int64(1))
	testutil.Eq(t, len(target.Comments), 1)

	src := getPost(t, app.Client(t), a.ID)
	if src.Post != nil || src.MergedInto == nil || src.MergedInto.ID != b.ID || src.MergedInto.Slug != b.Slug {
		t.Fatalf("source = %+v", src)
	}
	eqStrings(t, titles(list(t, app.Client(t), "").Posts), "Adagas duplas para assassin")
	eqStrings(t, titles(list(t, app.Client(t), "?q="+url.QueryEscape("dual daggers")).Posts))
	testutil.Eq(t, getPost(t, g1, b.ID).Post.Voted, true)
}

func TestMerge_Chain(t *testing.T) {
	app := testutil.New(t)
	author := app.Client(t)
	a, b, c := author.NewPost(t, "Post A"), author.NewPost(t, "Post B"), author.NewPost(t, "Post C")
	admin := app.AdminClient(t, "wuzi")
	merge(t, admin, a.ID, b.ID, 200)
	merge(t, admin, b.ID, c.ID, 200)
	got := getPost(t, app.Client(t), a.ID)
	testutil.Eq(t, got.MergedInto.ID, c.ID)
	var n int
	app.DB.QueryRow("SELECT count(*) FROM posts WHERE merged_into_id = ?", c.ID).Scan(&n)
	testutil.Eq(t, n, 2)
}

func TestMerge_Invalid(t *testing.T) {
	app := testutil.New(t)
	author := app.Client(t)
	a, b, c, d := author.NewPost(t, "Post A"), author.NewPost(t, "Post B"), author.NewPost(t, "Post C"), author.NewPost(t, "Post D")
	admin := app.AdminClient(t, "wuzi")
	testutil.Eq(t, admin.Do("POST", adminPath(a.ID, "merge"), map[string]int64{"intoId": a.ID}).Want(t, 422).ErrCode(t), "invalid_merge")
	merge(t, admin, b.ID, c.ID, 200)
	merge(t, admin, a.ID, b.ID, 422)
	merge(t, admin, b.ID, a.ID, 422)
	testutil.Exec(t, app.DB, "UPDATE posts SET deleted_at = 1 WHERE id = ?", d.ID)
	merge(t, admin, d.ID, a.ID, 422)
	merge(t, admin, a.ID, d.ID, 422)
	merge(t, admin, a.ID, 999, 422)
	app.Client(t).Do("POST", adminPath(a.ID, "merge"), map[string]int64{"intoId": c.ID}).Want(t, 401)
}
