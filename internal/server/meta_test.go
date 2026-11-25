package server_test

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"net/http"
	"strings"
	"testing"

	"patchboard/internal/api"
	"patchboard/internal/testutil"
)

func page(t *testing.T, c *testutil.Client, path string, _ string) string {
	t.Helper()
	r := c.Do("GET", path, nil).Want(t, 200)
	return string(r.Body)
}

func ogTitle(body string) string {
	const k = `<meta property="og:title" content="`
	i := strings.Index(body, k)
	if i < 0 {
		return ""
	}
	rest := body[i+len(k):]
	return rest[:strings.Index(rest, `"`)]
}

func TestMeta_Post(t *testing.T) {
	app := testutil.New(t)
	c := app.Client(t)
	var buf bytes.Buffer
	jpeg.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 20, 20)), nil)
	var img api.Image
	c.Upload("/api/uploads", "file", "a.jpg", buf.Bytes()).Want(t, 201).JSON(t, &img)
	var p api.PostDetail
	c.Do("POST", "/api/posts", map[string]any{"title": "Aumentar <tamanho> do chat", "body": "Por favor", "imageIds": []string{img.ID}}).Want(t, 201).JSON(t, &p)
	body := page(t, c, fmt.Sprintf("/p/%d-anything", p.ID), "")
	testutil.Eq(t, ogTitle(body), "Aumentar &lt;tamanho&gt; do chat")
	if !strings.Contains(body, fmt.Sprintf(`<meta property="og:url" content="http://pb.test/p/%d-%s">`, p.ID, p.Slug)) {
		t.Fatalf("og:url missing: %s", body)
	}
	if !strings.Contains(body, `<meta property="og:image" content="http://pb.test`+img.URL+`">`) {
		t.Fatalf("og:image missing: %s", body)
	}
	if !strings.Contains(body, `<meta property="og:description" content="Por favor">`) || !strings.Contains(body, "summary_large_image") {
		t.Fatalf("description/card missing: %s", body)
	}
}

func TestMeta_HiddenPostUsesBoardDefaults(t *testing.T) {
	app := testutil.New(t)
	testutil.Exec(t, app.DB, "UPDATE board_settings SET title_pt = 'Venore Survivors • Comunidade'")
	author := app.Client(t)
	pending, deleted, merged, target := author.NewPost(t, "Segredo pendente"), author.NewPost(t, "Segredo apagado"), author.NewPost(t, "Segredo fundido"), author.NewPost(t, "Alvo")
	testutil.Exec(t, app.DB, "UPDATE posts SET pending = 1 WHERE id = ?", pending.ID)
	testutil.Exec(t, app.DB, "UPDATE posts SET deleted_at = 1 WHERE id = ?", deleted.ID)
	testutil.Exec(t, app.DB, "UPDATE posts SET merged_into_id = ? WHERE id = ?", target.ID, merged.ID)
	for _, id := range []int64{pending.ID, deleted.ID, merged.ID, 999} {
		body := page(t, author, fmt.Sprintf("/p/%d-x", id), "")
		testutil.Eq(t, ogTitle(body), "Venore Survivors • Comunidade")
		if strings.Contains(body, "Segredo") {
			t.Fatalf("hidden title leaked for post %d", id)
		}
	}
}

func TestMeta_Language(t *testing.T) {
	app := testutil.New(t)
	testutil.Exec(t, app.DB, "UPDATE board_settings SET title_pt = 'Comunidade', title_en = 'Community'")
	c := app.Client(t)
	testutil.Eq(t, ogTitle(page(t, c, "/roadmap", "")), "Comunidade")
	req, _ := newGet(app, "/roadmap")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	testutil.Eq(t, ogTitle(string(c.Send(req).Want(t, 200).Body)), "Community")
	req, _ = newGet(app, "/roadmap")
	req.Header.Set("Accept-Language", "pt-BR,pt;q=0.9,en;q=0.8")
	testutil.Eq(t, ogTitle(string(c.Send(req).Want(t, 200).Body)), "Comunidade")
}

func newGet(app *testutil.App, path string) (*http.Request, error) {
	return http.NewRequest("GET", app.URL+path, nil)
}

func TestAPIStillJSON404(t *testing.T) {
	app := testutil.New(t)
	r := app.Client(t).Do("GET", "/api/nope", nil).Want(t, 404)
	testutil.Eq(t, r.ErrCode(t), "not_found")
}
