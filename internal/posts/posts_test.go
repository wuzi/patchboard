package posts_test

import (
	"strconv"
	"strings"
	"testing"

	"patchboard/internal/api"
	"patchboard/internal/testutil"
)

func path(id int64) string { return "/api/posts/" + strconv.FormatInt(id, 10) }

func getPost(t *testing.T, c *testutil.Client, id int64) api.PostResponse {
	t.Helper()
	var r api.PostResponse
	c.Do("GET", path(id), nil).Want(t, 200).JSON(t, &r)
	return r
}

func enableApproval(t *testing.T, app *testutil.App) {
	testutil.Exec(t, app.DB, "UPDATE board_settings SET require_approval = 1")
}

func TestCreatePost_Guest(t *testing.T) {
	app := testutil.New(t)
	c := app.Client(t)
	var p api.PostDetail
	r := c.Do("POST", "/api/posts", map[string]any{"title": "Aumentar tamanho do chat", "body": "**pls**"}).Want(t, 201)
	r.JSON(t, &p)
	testutil.Eq(t, p.StatusID, int64(1))
	testutil.Eq(t, p.Slug, "aumentar-tamanho-do-chat")
	if !strings.Contains(p.BodyHTML, "<strong>pls</strong>") {
		t.Fatalf("bodyHtml = %q", p.BodyHTML)
	}
	testutil.Eq(t, p.BodyMD, "**pls**")
	testutil.Eq(t, p.Excerpt, "pls")
	testutil.Eq(t, p.Author.Kind, "guest")
	testutil.Eq(t, p.CanEdit, true)
	testutil.Eq(t, p.CanDelete, true)
	testutil.Eq(t, p.VoteCount, int64(0))
	testutil.Eq(t, p.Pending, false)
	if p.Comments == nil || p.Images == nil || p.Voters == nil {
		t.Fatal("slices must be [] not null")
	}
	if r.SetCookie("pb_guest") == nil {
		t.Fatal("guest cookie not set")
	}
	// another viewer sees the post without the markdown source and without edit rights
	other := getPost(t, app.Client(t), p.ID).Post
	testutil.Eq(t, other.BodyMD, "")
	testutil.Eq(t, other.CanEdit, false)
}

func TestCreatePost_Validation(t *testing.T) {
	app := testutil.New(t)
	c := app.Client(t)
	testutil.Eq(t, c.Do("POST", "/api/posts", map[string]any{"title": "ab"}).Want(t, 422).Field(t, "title"), "too_short")
	testutil.Eq(t, c.Do("POST", "/api/posts", map[string]any{"title": strings.Repeat("a", 121)}).Want(t, 422).Field(t, "title"), "too_long")
	testutil.Eq(t, c.Do("POST", "/api/posts", map[string]any{"title": "Valid title", "body": strings.Repeat("a", 10001)}).Want(t, 422).Field(t, "body"), "too_long")
	testutil.Eq(t, c.Do("POST", "/api/posts", map[string]any{"title": "Valid title", "tagId": 999}).Want(t, 422).Field(t, "tagId"), "invalid")
	if c.Cookie("pb_guest") != nil {
		t.Fatal("invalid posts must not create a guest")
	}
	var p api.PostDetail
	c.Do("POST", "/api/posts", map[string]any{"title": "  Co-op  ", "tagId": 1}).Want(t, 201).JSON(t, &p)
	testutil.Eq(t, p.Title, "Co-op")
	testutil.Eq(t, *p.TagID, int64(1))
}

func TestCreatePost_Admin(t *testing.T) {
	app := testutil.New(t)
	c := app.AdminClient(t, "wuzi")
	var p api.PostDetail
	r := c.Do("POST", "/api/posts", map[string]any{"title": "Assassin dual daggers"}).Want(t, 201)
	r.JSON(t, &p)
	testutil.Eq(t, p.Author.Kind, "admin")
	testutil.Eq(t, p.Author.ID, "admin-1")
	testutil.Eq(t, *p.Author.Name, "Wuzi")
	if r.SetCookie("pb_guest") != nil {
		t.Fatal("admin posts must not create a guest")
	}
}

func TestCreatePost_RateLimit(t *testing.T) {
	app := testutil.New(t)
	c := app.Client(t)
	for i := 0; i < 5; i++ {
		c.Do("POST", "/api/posts", map[string]any{"title": "Post " + strconv.Itoa(i)}).Want(t, 201)
	}
	testutil.Eq(t, c.Do("POST", "/api/posts", map[string]any{"title": "Post 6"}).Want(t, 429).ErrCode(t), "rate_limited")
	admin := app.AdminClient(t, "wuzi")
	for i := 0; i < 6; i++ {
		admin.Do("POST", "/api/posts", map[string]any{"title": "Admin post " + strconv.Itoa(i)}).Want(t, 201)
	}
}

func TestCreatePost_Approval(t *testing.T) {
	app := testutil.New(t)
	enableApproval(t, app)
	author := app.Client(t)
	p := author.NewPost(t, "Filtro de Level no market")
	testutil.Eq(t, p.Pending, true)
	app.Client(t).Do("GET", path(p.ID), nil).Want(t, 404)
	testutil.Eq(t, getPost(t, author, p.ID).Post.Pending, true)
	admin := app.AdminClient(t, "wuzi")
	testutil.Eq(t, getPost(t, admin, p.ID).Post.Pending, true)
	var ap api.PostDetail
	admin.Do("POST", "/api/posts", map[string]any{"title": "Team post"}).Want(t, 201).JSON(t, &ap)
	testutil.Eq(t, ap.Pending, false)
}

func TestEditPost_Window(t *testing.T) {
	app := testutil.New(t)
	author := app.Client(t)
	p := author.NewPost(t, "Aumentar tamanho do chat")
	var e api.PostDetail
	author.Do("PATCH", path(p.ID), map[string]any{"title": "Aumentar o chat", "body": "_maior_", "tagId": 2}).Want(t, 200).JSON(t, &e)
	testutil.Eq(t, e.Slug, "aumentar-o-chat")
	testutil.Eq(t, *e.TagID, int64(2))
	if !strings.Contains(e.BodyHTML, "<em>maior</em>") {
		t.Fatalf("bodyHtml = %q", e.BodyHTML)
	}
	testutil.Eq(t, app.Client(t).Do("PATCH", path(p.ID), map[string]any{"title": "Hijack"}).Want(t, 403).ErrCode(t), "forbidden")

	testutil.Exec(t, app.DB, "UPDATE posts SET status_id = 3 WHERE id = ?", p.ID)
	testutil.Eq(t, author.Do("PATCH", path(p.ID), map[string]any{"title": "Too late"}).Want(t, 403).ErrCode(t), "edit_window_closed")
	testutil.Eq(t, getPost(t, author, p.ID).Post.CanEdit, false)
	admin := app.AdminClient(t, "wuzi")
	admin.Do("PATCH", path(p.ID), map[string]any{"clearTag": true}).Want(t, 200).JSON(t, &e)
	if e.TagID != nil {
		t.Fatalf("tag not cleared: %v", *e.TagID)
	}
	testutil.Eq(t, admin.Do("PATCH", path(p.ID), map[string]any{"title": "x"}).Want(t, 422).Field(t, "title"), "too_short")
}

func TestDeletePost(t *testing.T) {
	app := testutil.New(t)
	author := app.Client(t)
	p := author.NewPost(t, "Armas elementais para vocações melee")
	app.Client(t).Do("DELETE", path(p.ID), nil).Want(t, 403)
	author.Do("DELETE", path(p.ID), nil).Want(t, 204)
	author.Do("GET", path(p.ID), nil).Want(t, 404)
	app.Client(t).Do("GET", path(p.ID), nil).Want(t, 404)
	admin := app.AdminClient(t, "wuzi")
	testutil.Eq(t, getPost(t, admin, p.ID).Post.Deleted, true)

	q := author.NewPost(t, "Ver cooldown de Revelation Perk")
	admin.Do("DELETE", path(q.ID), nil).Want(t, 204)
	var by int64
	app.DB.QueryRow("SELECT deleted_by_admin_id FROM posts WHERE id = ?", q.ID).Scan(&by)
	if by == 0 {
		t.Fatal("admin delete must record deleted_by_admin_id")
	}
}

func TestGetPost_BadID(t *testing.T) {
	app := testutil.New(t)
	c := app.Client(t)
	c.Do("GET", "/api/posts/abc", nil).Want(t, 404)
	c.Do("GET", "/api/posts/999", nil).Want(t, 404)
}
