package posts_test

import (
	"fmt"
	"testing"
	"time"

	"patchboard/internal/api"
	"patchboard/internal/testutil"
)

func flagsOf(audit api.VoteAudit, guestID string) []string {
	for _, v := range audit.Voters {
		if v.Guest.ID == guestID {
			return v.Flags
		}
	}
	return nil
}

func has(flags []string, f string) bool {
	for _, x := range flags {
		if x == f {
			return true
		}
	}
	return false
}

func guestID(t *testing.T, c *testutil.Client) string {
	t.Helper()
	var m api.Me
	c.Do("GET", "/api/me", nil).Want(t, 200).JSON(t, &m)
	return m.Guest.ID
}

func TestVoteAudit(t *testing.T) {
	app := testutil.New(t)
	admin := app.AdminClient(t, "wuzi")
	p := admin.NewPost(t, "Assassin dual daggers")
	old := app.Client(t)
	old.Do("PUT", "/api/me/name", map[string]string{"name": "Veterano"}).Want(t, 200)
	app.Clock.Advance(2 * time.Hour)
	old.Vote(t, p.ID)
	var shared []*testutil.Client
	for i := 0; i < 3; i++ {
		c := app.Client(t).WithIP("203.0.113.80")
		c.Vote(t, p.ID)
		shared = append(shared, c)
	}
	late := app.Client(t)
	late.Do("PUT", "/api/me/name", map[string]string{"name": "Novato"}).Want(t, 200)
	app.Clock.Advance(5 * time.Minute)
	late.Vote(t, p.ID)

	var audit api.VoteAudit
	admin.Do("GET", fmt.Sprintf("/api/admin/posts/%d/votes", p.ID), nil).Want(t, 200).JSON(t, &audit)
	testutil.Eq(t, len(audit.Voters), 5)
	for _, c := range shared {
		if !has(flagsOf(audit, guestID(t, c)), "shared_ip") {
			t.Fatalf("shared-IP voter not flagged: %v", flagsOf(audit, guestID(t, c)))
		}
	}
	testutil.Eq(t, len(flagsOf(audit, guestID(t, old))), 0)
	if !has(flagsOf(audit, guestID(t, late)), "new_guest") {
		t.Fatalf("new guest not flagged: %v", flagsOf(audit, guestID(t, late)))
	}
	testutil.Eq(t, len(audit.Voters[0].IPHash), 8)
	app.Client(t).Do("GET", fmt.Sprintf("/api/admin/posts/%d/votes", p.ID), nil).Want(t, 401)
}

func TestRemoveVote(t *testing.T) {
	app := testutil.New(t)
	admin := app.AdminClient(t, "wuzi")
	p := admin.NewPost(t, "Co-op")
	g1, g2 := app.Client(t), app.Client(t)
	g1.Vote(t, p.ID)
	g2.Vote(t, p.ID)
	var v api.VoteState
	admin.Do("DELETE", fmt.Sprintf("/api/admin/posts/%d/votes/%s", p.ID, guestID(t, g1)), nil).Want(t, 200).JSON(t, &v)
	testutil.Eq(t, v, api.VoteState{VoteCount: 1, Voted: false})
	testutil.Eq(t, getPost(t, g1, p.ID).Post.Voted, false)
	testutil.Eq(t, getPost(t, g2, p.ID).Post.Voted, true)
}
