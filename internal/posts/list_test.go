package posts_test

import (
	"strconv"
	"testing"
	"time"

	"patchboard/internal/api"
	"patchboard/internal/testutil"
)

func list(t *testing.T, c *testutil.Client, query string) api.PostPage {
	t.Helper()
	var p api.PostPage
	c.Do("GET", "/api/posts"+query, nil).Want(t, 200).JSON(t, &p)
	return p
}

func titles(posts []api.PostCard) []string {
	out := make([]string, len(posts))
	for i, p := range posts {
		out[i] = p.Title
	}
	return out
}

func eqStrings(t *testing.T, got []string, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func votes(t *testing.T, app *testutil.App, id int64, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		app.Client(t).Vote(t, id)
	}
}

// seedABC creates A, B, C (in that order, one second apart); A gets 3 old votes, B 2 and C 1 recent ones.
func seedABC(t *testing.T, app *testutil.App) (a, b, c api.PostDetail) {
	author := app.AdminClient(t, "wuzi")
	a = author.NewPost(t, "Post A")
	app.Clock.Advance(time.Second)
	b = author.NewPost(t, "Post B")
	app.Clock.Advance(time.Second)
	c = author.NewPost(t, "Post C")
	votes(t, app, a.ID, 3)
	app.Clock.Advance(8 * 24 * time.Hour)
	votes(t, app, b.ID, 2)
	votes(t, app, c.ID, 1)
	return
}

func TestList_Sorts(t *testing.T) {
	app := testutil.New(t)
	seedABC(t, app)
	c := app.Client(t)
	trending := list(t, c, "")
	eqStrings(t, titles(trending.Posts), "Post B", "Post C", "Post A")
	testutil.Eq(t, trending.Posts[0].NewVotes, int64(2))
	testutil.Eq(t, trending.Posts[2].NewVotes, int64(0))
	testutil.Eq(t, trending.Posts[2].VoteCount, int64(3))
	eqStrings(t, titles(list(t, c, "?sort=top").Posts), "Post A", "Post B", "Post C")
	eqStrings(t, titles(list(t, c, "?sort=new").Posts), "Post C", "Post B", "Post A")
	eqStrings(t, titles(list(t, c, "?sort=bogus").Posts), "Post B", "Post C", "Post A")
}

func TestList_PinnedFirst(t *testing.T) {
	app := testutil.New(t)
	a, _, _ := seedABC(t, app)
	testutil.Exec(t, app.DB, "UPDATE posts SET pinned_at = 1 WHERE id = ?", a.ID)
	got := list(t, app.Client(t), "")
	eqStrings(t, titles(got.Posts), "Post A", "Post B", "Post C")
	testutil.Eq(t, got.Posts[0].Pinned, true)
	testutil.Eq(t, got.Posts[1].Pinned, false)
}

func TestList_Filters(t *testing.T) {
	app := testutil.New(t)
	a, b, _ := seedABC(t, app)
	testutil.Exec(t, app.DB, "UPDATE posts SET status_id = 2 WHERE id = ?", a.ID)
	testutil.Exec(t, app.DB, "UPDATE posts SET tag_id = 1 WHERE id = ?", b.ID)
	c := app.Client(t)
	eqStrings(t, titles(list(t, c, "?status=2").Posts), "Post A")
	eqStrings(t, titles(list(t, c, "?tag=1").Posts), "Post B")
	testutil.Eq(t, c.Do("GET", "/api/posts?status=x", nil).Want(t, 422).Field(t, "status"), "invalid")
	testutil.Eq(t, c.Do("GET", "/api/posts?tag=x", nil).Want(t, 422).Field(t, "tag"), "invalid")
}

func TestList_ExcludesHidden(t *testing.T) {
	app := testutil.New(t)
	author := app.Client(t)
	visible := author.NewPost(t, "Visible")
	pending, deleted, merged := author.NewPost(t, "Pending"), author.NewPost(t, "Deleted"), author.NewPost(t, "Merged")
	testutil.Exec(t, app.DB, "UPDATE posts SET pending = 1 WHERE id = ?", pending.ID)
	testutil.Exec(t, app.DB, "UPDATE posts SET deleted_at = 1 WHERE id = ?", deleted.ID)
	testutil.Exec(t, app.DB, "UPDATE posts SET merged_into_id = ? WHERE id = ?", visible.ID, merged.ID)
	eqStrings(t, titles(list(t, author, "").Posts), "Visible")
	var rm api.Roadmap
	author.Do("GET", "/api/roadmap", nil).Want(t, 200).JSON(t, &rm)
	for _, col := range rm.Columns {
		for _, p := range col.Posts {
			if p.ID != visible.ID {
				t.Fatalf("hidden post %q on roadmap", p.Title)
			}
		}
	}
}

func TestList_VotedAndVoters(t *testing.T) {
	app := testutil.New(t)
	_, b, _ := seedABC(t, app)
	viewer := app.Client(t)
	viewer.Do("PUT", "/api/me/name", map[string]string{"name": "Viewer"}).Want(t, 200)
	app.Clock.Advance(time.Second)
	viewer.Vote(t, b.ID)
	app.Clock.Advance(time.Second)
	app.Client(t).Vote(t, b.ID)
	got := list(t, viewer, "?sort=top")
	var pb, pc api.PostCard
	for _, p := range got.Posts {
		switch p.Title {
		case "Post B":
			pb = p
		case "Post C":
			pc = p
		}
	}
	testutil.Eq(t, pb.Voted, true)
	testutil.Eq(t, pc.Voted, false)
	testutil.Eq(t, len(pb.Voters), 3)
	testutil.Eq(t, pb.Voters[1].Kind, "guest")
	testutil.Eq(t, *pb.Voters[1].Name, "Viewer")
	if pb.Voters[0].Name != nil {
		t.Fatalf("most recent voter should be the unnamed guest, got %v", *pb.Voters[0].Name)
	}
}

func TestList_Pagination(t *testing.T) {
	app := testutil.New(t)
	admin := app.AdminClient(t, "wuzi")
	for i := 0; i < 60; i++ {
		admin.NewPost(t, "Post number "+strconv.Itoa(i))
	}
	c := app.Client(t)
	page := list(t, c, "")
	testutil.Eq(t, len(page.Posts), 30)
	testutil.Eq(t, *page.NextOffset, 30)
	page = list(t, c, "?offset=55")
	testutil.Eq(t, len(page.Posts), 5)
	if page.NextOffset != nil {
		t.Fatalf("nextOffset = %d", *page.NextOffset)
	}
	page = list(t, c, "?limit=100")
	testutil.Eq(t, len(page.Posts), 50)
	testutil.Eq(t, *page.NextOffset, 50)
}

func TestRoadmap(t *testing.T) {
	app := testutil.New(t)
	a, b, c := seedABC(t, app)
	testutil.Exec(t, app.DB, "UPDATE posts SET status_id = 3 WHERE id IN (?, ?)", a.ID, c.ID)
	_ = b
	viewer := app.Client(t)
	var rm api.Roadmap
	viewer.Do("GET", "/api/roadmap", nil).Want(t, 200).JSON(t, &rm)
	testutil.Eq(t, len(rm.Columns), 6)
	testutil.Eq(t, rm.Columns[0].StatusID, int64(1))
	testutil.Eq(t, rm.Columns[2].StatusID, int64(3))
	testutil.Eq(t, rm.Columns[2].Total, int64(2))
	eqStrings(t, titles(rm.Columns[2].Posts), "Post A", "Post C")
	testutil.Eq(t, rm.Columns[0].Total, int64(1))
	testutil.Eq(t, len(rm.Columns[3].Posts), 0)

	app.AdminClient(t, "mod").Do("PATCH", "/api/admin/statuses/6", map[string]any{"showOnRoadmap": false}).Want(t, 200)
	viewer.Do("GET", "/api/roadmap", nil).Want(t, 200).JSON(t, &rm)
	testutil.Eq(t, len(rm.Columns), 5)

	testutil.Exec(t, app.DB, "UPDATE posts SET tag_id = 2 WHERE id = ?", c.ID)
	viewer.Do("GET", "/api/roadmap?tag=2", nil).Want(t, 200).JSON(t, &rm)
	eqStrings(t, titles(rm.Columns[2].Posts), "Post C")
	testutil.Eq(t, rm.Columns[2].Total, int64(1))
}
