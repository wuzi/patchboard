package posts_test

import (
	"net/url"
	"strings"
	"testing"

	"patchboard/internal/api"
	"patchboard/internal/posts"
	"patchboard/internal/testutil"
)

func TestFTSQuery(t *testing.T) {
	testutil.Eq(t, posts.FTSQuery("Análise de dano", false), `"Análise"* "de"* "dano"*`)
	testutil.Eq(t, posts.FTSQuery(`"chat" OR (`, false), `"chat"* "OR"*`)
	testutil.Eq(t, posts.FTSQuery("ab cd", true), "")
	testutil.Eq(t, posts.FTSQuery("tamanho do chat", true), `"tamanho"* OR "chat"*`)
	testutil.Eq(t, posts.FTSQuery("   ", false), "")
	testutil.Eq(t, posts.FTSQuery("a b c d e f g h i j", false), `"a"* "b"* "c"* "d"* "e"* "f"* "g"* "h"*`)
}

func search(t *testing.T, c *testutil.Client, q string) []string {
	t.Helper()
	return titles(list(t, c, "?q="+url.QueryEscape(q)).Posts)
}

func TestSearch_AccentInsensitive(t *testing.T) {
	app := testutil.New(t)
	author := app.AdminClient(t, "wuzi")
	author.NewPost(t, "Melhorar análise de dano")
	author.NewPost(t, "Aumentar tamanho do chat")
	c := app.Client(t)
	eqStrings(t, search(t, c, "analise"), "Melhorar análise de dano")
	eqStrings(t, search(t, c, "ANÁL"), "Melhorar análise de dano")
	eqStrings(t, search(t, c, "dano"), "Melhorar análise de dano")
	eqStrings(t, search(t, c, "chat tamanho"), "Aumentar tamanho do chat")
	eqStrings(t, search(t, c, "chat xyz"))
	eqStrings(t, search(t, c, `") OR (`))
}

func TestSearch_MatchesBody(t *testing.T) {
	app := testutil.New(t)
	c := app.AdminClient(t, "wuzi")
	c.Do("POST", "/api/posts", map[string]any{"title": "Venda rápida", "body": "Na Loja de magia do Xodet"}).Want(t, 201)
	eqStrings(t, search(t, app.Client(t), "xodet"), "Venda rápida")
}

func TestSearch_AppliesToRoadmap(t *testing.T) {
	app := testutil.New(t)
	author := app.AdminClient(t, "wuzi")
	author.NewPost(t, "Aumentar tamanho do chat")
	author.NewPost(t, "Co-op")
	var rm api.Roadmap
	app.Client(t).Do("GET", "/api/roadmap?q=chat", nil).Want(t, 200).JSON(t, &rm)
	eqStrings(t, titles(rm.Columns[0].Posts), "Aumentar tamanho do chat")
	testutil.Eq(t, rm.Columns[0].Total, int64(1))
}

func similar(t *testing.T, c *testutil.Client, title string) []string {
	t.Helper()
	var s api.Similar
	c.Do("GET", "/api/posts/similar?title="+url.QueryEscape(title), nil).Want(t, 200).JSON(t, &s)
	out := make([]string, len(s.Posts))
	for i, p := range s.Posts {
		out[i] = p.Title
	}
	return out
}

func TestSimilar(t *testing.T) {
	app := testutil.New(t)
	author := app.AdminClient(t, "wuzi")
	author.NewPost(t, "Aumentar tamanho do chat")
	author.NewPost(t, "Co-op")
	c := app.Client(t)
	got := similar(t, c, "chat maior por favor")
	if len(got) == 0 || got[0] != "Aumentar tamanho do chat" {
		t.Fatalf("similar = %v", got)
	}
	eqStrings(t, similar(t, c, "ab"))
	for i := 0; i < 7; i++ {
		author.NewPost(t, "Chat variant "+strings.Repeat("x", i+1))
	}
	if n := len(similar(t, c, "chat")); n != 5 {
		t.Fatalf("similar returned %d posts, want 5", n)
	}
}

func TestSimilar_ExcludesHidden(t *testing.T) {
	app := testutil.New(t)
	author := app.Client(t)
	target := author.NewPost(t, "Filtro de level no market")
	pending, deleted, merged := author.NewPost(t, "Filtro pendente market"), author.NewPost(t, "Filtro apagado market"), author.NewPost(t, "Filtro fundido market")
	testutil.Exec(t, app.DB, "UPDATE posts SET pending = 1 WHERE id = ?", pending.ID)
	testutil.Exec(t, app.DB, "UPDATE posts SET deleted_at = 1 WHERE id = ?", deleted.ID)
	testutil.Exec(t, app.DB, "UPDATE posts SET merged_into_id = ? WHERE id = ?", target.ID, merged.ID)
	eqStrings(t, similar(t, app.Client(t), "filtro market"), "Filtro de level no market")
	eqStrings(t, search(t, app.Client(t), "filtro"), "Filtro de level no market")
}
