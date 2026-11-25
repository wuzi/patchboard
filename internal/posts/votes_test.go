package posts_test

import (
	"net/netip"
	"sync"
	"testing"

	"patchboard/internal/api"
	"patchboard/internal/httpx"
	"patchboard/internal/testutil"
)

func vote(t *testing.T, c *testutil.Client, method string, id int64, status int) api.VoteState {
	t.Helper()
	var v api.VoteState
	r := c.Do(method, path(id)+"/vote", nil).Want(t, status)
	if status == 200 {
		r.JSON(t, &v)
	}
	return v
}

func TestVote_Toggle(t *testing.T) {
	app := testutil.New(t)
	p := app.Client(t).NewPost(t, "Co-op")
	c := app.Client(t)
	vote(t, c, "POST", p.ID, 200)
	testutil.Eq(t, vote(t, c, "POST", p.ID, 200), api.VoteState{VoteCount: 1, Voted: true})
	testutil.Eq(t, getPost(t, c, p.ID).Post.Voted, true)
	testutil.Eq(t, vote(t, c, "DELETE", p.ID, 200), api.VoteState{VoteCount: 0, Voted: false})
	testutil.Eq(t, vote(t, c, "DELETE", p.ID, 200), api.VoteState{VoteCount: 0, Voted: false})
}

func TestVote_HiddenPosts(t *testing.T) {
	app := testutil.New(t)
	author := app.Client(t)
	pending, deleted, merged, target := author.NewPost(t, "Pending"), author.NewPost(t, "Deleted"), author.NewPost(t, "Merged"), author.NewPost(t, "Target")
	testutil.Exec(t, app.DB, "UPDATE posts SET pending = 1 WHERE id = ?", pending.ID)
	testutil.Exec(t, app.DB, "UPDATE posts SET deleted_at = 1 WHERE id = ?", deleted.ID)
	testutil.Exec(t, app.DB, "UPDATE posts SET merged_into_id = ? WHERE id = ?", target.ID, merged.ID)
	c := app.Client(t)
	for _, id := range []int64{pending.ID, deleted.ID, merged.ID, 999} {
		vote(t, c, "POST", id, 404)
	}
}

func TestVote_Banned(t *testing.T) {
	app := testutil.New(t)
	p := app.Client(t).NewPost(t, "Co-op")
	c := app.Client(t)
	vote(t, c, "POST", p.ID, 200)
	var me api.Me
	c.Do("GET", "/api/me", nil).JSON(t, &me)
	testutil.Exec(t, app.DB, `INSERT INTO admins (id, username, display_name, password_hash, created_at) VALUES (99, 'mod', 'Mod', 'x', 0)`)
	testutil.Exec(t, app.DB, `INSERT INTO bans (guest_id, created_by, created_at) VALUES (?, 99, 0)`, me.Guest.ID)
	testutil.Eq(t, c.Do("DELETE", path(p.ID)+"/vote", nil).Want(t, 403).ErrCode(t), "banned")
}

func TestVote_RateLimit(t *testing.T) {
	app := testutil.New(t)
	p := app.Client(t).NewPost(t, "Co-op")
	c := app.Client(t)
	for i := 0; i < 120; i++ {
		method := "POST"
		if i%2 == 1 {
			method = "DELETE"
		}
		vote(t, c, method, p.ID, 200)
	}
	testutil.Eq(t, c.Do("POST", path(p.ID)+"/vote", nil).Want(t, 429).ErrCode(t), "rate_limited")
}

func TestVote_AdminWithoutGuestCookie(t *testing.T) {
	app := testutil.New(t)
	admin := app.AdminClient(t, "wuzi")
	p := admin.NewPost(t, "Transferência de tiers")
	if admin.Cookie("pb_guest") != nil {
		t.Fatal("admin should start without a guest cookie")
	}
	testutil.Eq(t, vote(t, admin, "POST", p.ID, 200).Voted, true)
	if admin.Cookie("pb_guest") == nil {
		t.Fatal("voting must give the admin a guest identity")
	}
	testutil.Eq(t, getPost(t, admin, p.ID).Post.VoteCount, int64(1))
}

func TestVote_Concurrent(t *testing.T) {
	app := testutil.New(t)
	p := app.Client(t).NewPost(t, "Aumentar tamanho do chat")
	clients := make([]*testutil.Client, 20)
	for i := range clients {
		clients[i] = app.Client(t)
	}
	codes := make(chan int, len(clients))
	var wg sync.WaitGroup
	for _, c := range clients {
		wg.Add(1)
		go func(c *testutil.Client) {
			defer wg.Done()
			codes <- c.Do("POST", path(p.ID)+"/vote", nil).Code
		}(c)
	}
	wg.Wait()
	close(codes)
	for code := range codes {
		testutil.Eq(t, code, 200)
	}
	testutil.Eq(t, getPost(t, app.Client(t), p.ID).Post.VoteCount, int64(20))
}

func TestVote_RecordsIPHash(t *testing.T) {
	app := testutil.New(t)
	p := app.Client(t).NewPost(t, "Co-op")
	c := app.Client(t)
	vote(t, c, "POST", p.ID, 200)
	var h string
	if err := app.DB.QueryRow("SELECT ip_hash FROM votes WHERE post_id = ?", p.ID).Scan(&h); err != nil {
		t.Fatal(err)
	}
	testutil.Eq(t, h, httpx.NewIPHasher(app.Cfg.Secret).Hash(netip.MustParseAddr(c.IP)))
}
