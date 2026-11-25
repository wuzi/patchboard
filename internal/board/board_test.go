package board_test

import (
	"strconv"
	"strings"
	"testing"

	"patchboard/internal/api"
	"patchboard/internal/testutil"
)

func getBoard(t *testing.T, c *testutil.Client) api.Board {
	t.Helper()
	var b api.Board
	c.Do("GET", "/api/board", nil).Want(t, 200).JSON(t, &b)
	return b
}

func validSettings() map[string]any {
	return map[string]any{
		"title":           map[string]string{"pt": "Venore Survivors • Comunidade", "en": "Venore Survivors • Community"},
		"description":     map[string]string{"pt": "Board oficial", "en": "Official board"},
		"teamName":        "Equipe Venore Survivors",
		"accentColor":     "#2563EB",
		"logoUploadId":    nil,
		"requireApproval": true,
		"uploadsEnabled":  false,
	}
}

func TestBoard_Public(t *testing.T) {
	app := testutil.New(t)
	b := getBoard(t, app.Client(t))
	testutil.Eq(t, len(b.Statuses), 6)
	testutil.Eq(t, b.Statuses[0].Name, api.I18nName{PT: "Sugestões", EN: "Suggestions"})
	testutil.Eq(t, b.Statuses[0].IsDefault, true)
	testutil.Eq(t, b.Statuses[5].Name.PT, "Recusado")
	testutil.Eq(t, len(b.Tags), 3)
	testutil.Eq(t, b.Tags[1].Name, api.I18nName{PT: "Melhoria", EN: "Improvement"})
	testutil.Eq(t, b.AccentColor, "#c026d3")
	if b.LogoURL != nil {
		t.Fatalf("logoUrl = %v", *b.LogoURL)
	}
}

func TestSettings_Update(t *testing.T) {
	app := testutil.New(t)
	admin := app.AdminClient(t, "wuzi")
	admin.Do("PUT", "/api/admin/settings", validSettings()).Want(t, 200)
	b := getBoard(t, app.Client(t))
	testutil.Eq(t, b.Title.PT, "Venore Survivors • Comunidade")
	testutil.Eq(t, b.TeamName, "Equipe Venore Survivors")
	testutil.Eq(t, b.AccentColor, "#2563EB")
	testutil.Eq(t, b.RequireApproval, true)
	testutil.Eq(t, b.UploadsEnabled, false)

	s := validSettings()
	s["accentColor"] = "red"
	testutil.Eq(t, admin.Do("PUT", "/api/admin/settings", s).Want(t, 422).Field(t, "accentColor"), "invalid")
	s = validSettings()
	s["title"] = map[string]string{"pt": "", "en": strings.Repeat("x", 81)}
	r := admin.Do("PUT", "/api/admin/settings", s).Want(t, 422)
	testutil.Eq(t, r.Field(t, "title.pt"), "required")
	testutil.Eq(t, r.Field(t, "title.en"), "too_long")
	s = validSettings()
	s["logoUploadId"] = "nope"
	testutil.Eq(t, admin.Do("PUT", "/api/admin/settings", s).Want(t, 422).Field(t, "logoUploadId"), "invalid")
	app.Client(t).Do("PUT", "/api/admin/settings", validSettings()).Want(t, 401)
}

func TestStatuses_CreateOrderDefault(t *testing.T) {
	app := testutil.New(t)
	admin := app.AdminClient(t, "wuzi")
	var st api.Status
	admin.Do("POST", "/api/admin/statuses", map[string]any{
		"name": map[string]string{"pt": "Em teste", "en": "Testing"}, "color": "#0EA5E9", "showOnRoadmap": true,
	}).Want(t, 201).JSON(t, &st)
	testutil.Eq(t, st.Position, int64(6))

	b := getBoard(t, admin)
	ids := make([]int64, 0, len(b.Statuses))
	for i := len(b.Statuses) - 1; i >= 0; i-- {
		ids = append(ids, b.Statuses[i].ID)
	}
	admin.Do("PUT", "/api/admin/statuses/order", map[string]any{"ids": ids}).Want(t, 200)
	b = getBoard(t, admin)
	for i, s := range b.Statuses {
		testutil.Eq(t, s.ID, ids[i])
		testutil.Eq(t, s.Position, int64(i))
	}
	testutil.Eq(t, admin.Do("PUT", "/api/admin/statuses/order", map[string]any{"ids": ids[1:]}).Want(t, 422).ErrCode(t), "invalid_order")

	admin.Do("PATCH", "/api/admin/statuses/3", map[string]any{"isDefault": true}).Want(t, 200)
	for _, s := range getBoard(t, admin).Statuses {
		testutil.Eq(t, s.IsDefault, s.ID == 3)
	}
	admin.Do("PATCH", "/api/admin/statuses/2", map[string]any{"name": map[string]string{"pt": "Análise", "en": "Review"}, "color": "#000000", "showOnRoadmap": false}).Want(t, 200)
	for _, s := range getBoard(t, admin).Statuses {
		if s.ID == 2 && (s.Name.PT != "Análise" || s.Color != "#000000" || s.ShowOnRoadmap) {
			t.Fatalf("status 2 = %+v", s)
		}
	}
	admin.Do("PATCH", "/api/admin/statuses/99", map[string]any{"color": "#000000"}).Want(t, 404)
}

func TestStatuses_Delete(t *testing.T) {
	app := testutil.New(t)
	admin := app.AdminClient(t, "wuzi")
	testutil.Eq(t, admin.Do("DELETE", "/api/admin/statuses/1", nil).Want(t, 422).ErrCode(t), "cannot_delete_default")
	post := testutil.InsertPost(t, app.DB, "Co-op", 2)
	testutil.Eq(t, admin.Do("DELETE", "/api/admin/statuses/2", nil).Want(t, 422).ErrCode(t), "move_required")
	admin.Do("DELETE", "/api/admin/statuses/2?moveTo=2", nil).Want(t, 422)
	admin.Do("DELETE", "/api/admin/statuses/2?moveTo=3", nil).Want(t, 204)
	var status int64
	if err := app.DB.QueryRow("SELECT status_id FROM posts WHERE id = ?", post).Scan(&status); err != nil {
		t.Fatal(err)
	}
	testutil.Eq(t, status, int64(3))
	admin.Do("DELETE", "/api/admin/statuses/4", nil).Want(t, 204)
	testutil.Eq(t, len(getBoard(t, admin).Statuses), 4)
}

func TestTags_CRUD(t *testing.T) {
	app := testutil.New(t)
	admin := app.AdminClient(t, "wuzi")
	var tag api.Tag
	admin.Do("POST", "/api/admin/tags", map[string]any{"name": map[string]string{"pt": "Balanceamento", "en": "Balance"}}).Want(t, 201).JSON(t, &tag)
	testutil.Eq(t, tag.Position, int64(3))
	admin.Do("PATCH", "/api/admin/tags/"+strconv.FormatInt(tag.ID, 10), map[string]any{"name": map[string]string{"pt": "Balanço", "en": "Balance"}}).Want(t, 200)
	admin.Do("PUT", "/api/admin/tags/order", map[string]any{"ids": []int64{tag.ID, 3, 2, 1}}).Want(t, 200)
	b := getBoard(t, admin)
	testutil.Eq(t, b.Tags[0].Name.PT, "Balanço")

	post := testutil.InsertPost(t, app.DB, "Bug no chat", 1)
	testutil.Exec(t, app.DB, "UPDATE posts SET tag_id = 3 WHERE id = ?", post)
	admin.Do("DELETE", "/api/admin/tags/3", nil).Want(t, 204)
	var n int
	app.DB.QueryRow("SELECT count(*) FROM posts WHERE id = ? AND tag_id IS NULL", post).Scan(&n)
	testutil.Eq(t, n, 1)

	r := admin.Do("POST", "/api/admin/tags", map[string]any{"name": map[string]string{"pt": "ok", "en": strings.Repeat("x", 31)}}).Want(t, 422)
	testutil.Eq(t, r.Field(t, "name.en"), "too_long")
}
