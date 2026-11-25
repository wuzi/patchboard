package comments_test

import (
	"fmt"
	"strings"
	"testing"

	"patchboard/internal/api"
	"patchboard/internal/testutil"
)

func commentsOf(t *testing.T, c *testutil.Client, postID int64) (api.PostDetail, []api.Comment) {
	t.Helper()
	var r api.PostResponse
	c.Do("GET", fmt.Sprintf("/api/posts/%d", postID), nil).Want(t, 200).JSON(t, &r)
	return *r.Post, r.Post.Comments
}

func comment(t *testing.T, c *testutil.Client, postID int64, body string, parent *int64) api.Comment {
	t.Helper()
	req := map[string]any{"body": body}
	if parent != nil {
		req["parentId"] = *parent
	}
	var out api.Comment
	c.Do("POST", fmt.Sprintf("/api/posts/%d/comments", postID), req).Want(t, 201).JSON(t, &out)
	return out
}

func TestComment_Thread(t *testing.T) {
	app := testutil.New(t)
	p := app.Client(t).NewPost(t, "Aumentar tamanho do chat")
	c := app.Client(t)
	c.Do("PUT", "/api/me/name", map[string]string{"name": "WuzeraProMAX"}).Want(t, 200)
	c1 := comment(t, c, p.ID, "**pls**, meu olhos agradecem", nil)
	testutil.Eq(t, *c1.Author.Name, "WuzeraProMAX")
	if !strings.Contains(*c1.BodyHTML, "<strong>pls</strong>") {
		t.Fatalf("bodyHtml = %q", *c1.BodyHTML)
	}
	r1 := comment(t, app.Client(t), p.ID, "concordo", &c1.ID)
	r2 := comment(t, app.Client(t), p.ID, "eu também", &r1.ID)
	testutil.Eq(t, *r2.ParentID, c1.ID)

	post, tree := commentsOf(t, app.Client(t), p.ID)
	testutil.Eq(t, post.CommentCount, int64(3))
	testutil.Eq(t, len(tree), 1)
	testutil.Eq(t, tree[0].ID, c1.ID)
	testutil.Eq(t, len(tree[0].Replies), 2)
	testutil.Eq(t, tree[0].Replies[0].ID, r1.ID)
	testutil.Eq(t, tree[0].Replies[1].ID, r2.ID)
	if tree[0].BodyMD != nil {
		t.Fatal("other viewers must not get the markdown source")
	}
	testutil.Eq(t, tree[0].CanEdit, false)
}

func TestComment_Locked(t *testing.T) {
	app := testutil.New(t)
	p := app.Client(t).NewPost(t, "Co-op")
	testutil.Exec(t, app.DB, "UPDATE posts SET comments_locked = 1 WHERE id = ?", p.ID)
	r := app.Client(t).Do("POST", fmt.Sprintf("/api/posts/%d/comments", p.ID), map[string]any{"body": "hi"}).Want(t, 403)
	testutil.Eq(t, r.ErrCode(t), "locked")
	c := comment(t, app.AdminClient(t, "wuzi"), p.ID, "Fechado por agora", nil)
	testutil.Eq(t, c.Author.Kind, "admin")
}

func TestComment_Validation(t *testing.T) {
	app := testutil.New(t)
	p := app.Client(t).NewPost(t, "Co-op")
	other := app.Client(t).NewPost(t, "Outro post")
	c := app.Client(t)
	path := fmt.Sprintf("/api/posts/%d/comments", p.ID)
	testutil.Eq(t, c.Do("POST", path, map[string]any{"body": "  "}).Want(t, 422).Field(t, "body"), "too_short")
	testutil.Eq(t, c.Do("POST", path, map[string]any{"body": strings.Repeat("a", 5001)}).Want(t, 422).Field(t, "body"), "too_long")
	foreign := comment(t, c, other.ID, "em outro post", nil)
	testutil.Eq(t, c.Do("POST", path, map[string]any{"body": "x", "parentId": foreign.ID}).Want(t, 422).Field(t, "parentId"), "invalid")
	testutil.Eq(t, c.Do("POST", path, map[string]any{"body": "x", "parentId": 9999}).Want(t, 422).Field(t, "parentId"), "invalid")
}

func TestComment_EditDelete(t *testing.T) {
	app := testutil.New(t)
	p := app.Client(t).NewPost(t, "Co-op")
	author := app.Client(t)
	c1 := comment(t, author, p.ID, "primeiro", nil)
	testutil.Eq(t, c1.CanEdit, true)
	testutil.Eq(t, *c1.BodyMD, "primeiro")
	r1 := comment(t, app.Client(t), p.ID, "resposta", &c1.ID)

	var edited api.Comment
	author.Do("PATCH", fmt.Sprintf("/api/comments/%d", c1.ID), map[string]any{"body": "_editado_"}).Want(t, 200).JSON(t, &edited)
	if !strings.Contains(*edited.BodyHTML, "<em>editado</em>") {
		t.Fatalf("bodyHtml = %q", *edited.BodyHTML)
	}
	app.Client(t).Do("PATCH", fmt.Sprintf("/api/comments/%d", c1.ID), map[string]any{"body": "hijack"}).Want(t, 403)
	app.Client(t).Do("DELETE", fmt.Sprintf("/api/comments/%d", c1.ID), nil).Want(t, 403)

	author.Do("DELETE", fmt.Sprintf("/api/comments/%d", c1.ID), nil).Want(t, 204)
	post, tree := commentsOf(t, app.Client(t), p.ID)
	testutil.Eq(t, post.CommentCount, int64(1))
	testutil.Eq(t, len(tree), 1)
	testutil.Eq(t, tree[0].Deleted, true)
	if tree[0].Author != nil || tree[0].BodyHTML != nil || tree[0].BodyMD != nil {
		t.Fatalf("deleted placeholder leaks content: %+v", tree[0])
	}
	testutil.Eq(t, len(tree[0].Replies), 1)

	app.AdminClient(t, "wuzi").Do("DELETE", fmt.Sprintf("/api/comments/%d", r1.ID), nil).Want(t, 204)
	post, tree = commentsOf(t, app.Client(t), p.ID)
	testutil.Eq(t, post.CommentCount, int64(0))
	testutil.Eq(t, len(tree), 0)
	author.Do("PATCH", fmt.Sprintf("/api/comments/%d", c1.ID), map[string]any{"body": "zombie"}).Want(t, 404)
}

func TestComment_Likes(t *testing.T) {
	app := testutil.New(t)
	p := app.Client(t).NewPost(t, "Co-op")
	c1 := comment(t, app.Client(t), p.ID, "gostei", nil)
	liker := app.Client(t)
	path := fmt.Sprintf("/api/comments/%d/like", c1.ID)
	var s api.LikeState
	liker.Do("POST", path, nil).Want(t, 200)
	liker.Do("POST", path, nil).Want(t, 200).JSON(t, &s)
	testutil.Eq(t, s, api.LikeState{LikeCount: 1, Liked: true})
	_, tree := commentsOf(t, liker, p.ID)
	testutil.Eq(t, tree[0].Liked, true)
	testutil.Eq(t, tree[0].LikeCount, int64(1))
	liker.Do("DELETE", path, nil).Want(t, 200).JSON(t, &s)
	testutil.Eq(t, s, api.LikeState{LikeCount: 0, Liked: false})

	testutil.Exec(t, app.DB, "UPDATE comments SET deleted_at = 1 WHERE id = ?", c1.ID)
	liker.Do("POST", path, nil).Want(t, 404)
}

func TestComment_HiddenPost(t *testing.T) {
	app := testutil.New(t)
	author := app.Client(t)
	pending := author.NewPost(t, "Pendente")
	deleted := author.NewPost(t, "Apagado")
	testutil.Exec(t, app.DB, "UPDATE posts SET pending = 1 WHERE id = ?", pending.ID)
	testutil.Exec(t, app.DB, "UPDATE posts SET deleted_at = 1 WHERE id = ?", deleted.ID)
	other := app.Client(t)
	other.Do("POST", fmt.Sprintf("/api/posts/%d/comments", pending.ID), map[string]any{"body": "x"}).Want(t, 404)
	other.Do("POST", fmt.Sprintf("/api/posts/%d/comments", deleted.ID), map[string]any{"body": "x"}).Want(t, 404)
	comment(t, author, pending.ID, "detalhe extra", nil)
}

func TestComment_RateLimit(t *testing.T) {
	app := testutil.New(t)
	p := app.Client(t).NewPost(t, "Co-op")
	c := app.Client(t)
	for i := 0; i < 20; i++ {
		comment(t, c, p.ID, fmt.Sprintf("comentário %d", i), nil)
	}
	testutil.Eq(t, c.Do("POST", fmt.Sprintf("/api/posts/%d/comments", p.ID), map[string]any{"body": "mais um"}).Want(t, 429).ErrCode(t), "rate_limited")
}
