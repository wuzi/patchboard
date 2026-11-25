package uploads_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"patchboard/internal/api"
	"patchboard/internal/testutil"
	"patchboard/internal/uploads"
)

func upload(t *testing.T, c *testutil.Client, data []byte) api.Image {
	t.Helper()
	var img api.Image
	c.Upload("/api/uploads", "file", "shot.jpg", data).Want(t, 201).JSON(t, &img)
	return img
}

func filePath(app *testutil.App, img api.Image) string {
	return filepath.Join(app.Cfg.DataDir, "uploads", filepath.Base(img.URL))
}

func TestUpload_Endpoint(t *testing.T) {
	app := testutil.New(t)
	c := app.Client(t)
	img := upload(t, c, jpegBytes(t, 200, 100))
	testutil.Eq(t, len(img.ID), 20)
	testutil.Eq(t, img.URL, "/uploads/"+img.ID+".jpg")
	testutil.Eq(t, img.Width, int64(200))
	testutil.Eq(t, img.Height, int64(100))
	if _, err := os.Stat(filePath(app, img)); err != nil {
		t.Fatalf("file not stored: %v", err)
	}
	testutil.Eq(t, c.Upload("/api/uploads", "file", "big.jpg", bytes.Repeat([]byte("a"), 6<<20)).Want(t, 413).ErrCode(t), "too_large")
	testutil.Eq(t, c.Upload("/api/uploads", "file", "x.txt", []byte("hello")).Want(t, 422).ErrCode(t), "invalid_image")
	testutil.Eq(t, c.Upload("/api/uploads", "file", "bomb.png", hugePNG(t, 20000, 20000)).Want(t, 422).ErrCode(t), "too_many_pixels")
}

func TestUpload_Disabled(t *testing.T) {
	app := testutil.New(t)
	testutil.Exec(t, app.DB, "UPDATE board_settings SET uploads_enabled = 0")
	testutil.Eq(t, app.Client(t).Upload("/api/uploads", "file", "a.jpg", jpegBytes(t, 10, 10)).Want(t, 403).ErrCode(t), "uploads_disabled")
	upload(t, app.AdminClient(t, "wuzi"), jpegBytes(t, 10, 10))
}

func TestUpload_RateLimit(t *testing.T) {
	app := testutil.New(t)
	c := app.Client(t)
	data := jpegBytes(t, 10, 10)
	for i := 0; i < 20; i++ {
		upload(t, c, data)
	}
	testutil.Eq(t, c.Upload("/api/uploads", "file", "a.jpg", data).Want(t, 429).ErrCode(t), "rate_limited")
}

func TestUpload_AttachToPost(t *testing.T) {
	app := testutil.New(t)
	c := app.Client(t)
	u1, u2 := upload(t, c, jpegBytes(t, 10, 10)), upload(t, c, pngBytes(t, 12, 12))
	var p api.PostDetail
	c.Do("POST", "/api/posts", map[string]any{"title": "Bug no inventário", "imageIds": []string{u1.ID, u2.ID}}).Want(t, 201).JSON(t, &p)
	testutil.Eq(t, len(p.Images), 2)
	testutil.Eq(t, p.Images[0].ID, u1.ID)
	testutil.Eq(t, p.Images[1].ID, u2.ID)

	r := c.Do("POST", "/api/posts", map[string]any{"title": "Too many", "imageIds": []string{"a", "b", "c", "d", "e"}}).Want(t, 422)
	testutil.Eq(t, r.Field(t, "imageIds"), "too_many")
	foreign := upload(t, app.Client(t), jpegBytes(t, 10, 10))
	testutil.Eq(t, c.Do("POST", "/api/posts", map[string]any{"title": "Foreign", "imageIds": []string{foreign.ID}}).Want(t, 422).Field(t, "imageIds"), "invalid")
	testutil.Eq(t, c.Do("POST", "/api/posts", map[string]any{"title": "Reused", "imageIds": []string{u1.ID}}).Want(t, 422).Field(t, "imageIds"), "invalid")

	c.Do("PATCH", fmt.Sprintf("/api/posts/%d", p.ID), map[string]any{"imageIds": []string{u2.ID}}).Want(t, 200).JSON(t, &p)
	testutil.Eq(t, len(p.Images), 1)
	testutil.Eq(t, p.Images[0].ID, u2.ID)
	var n int
	app.DB.QueryRow("SELECT count(*) FROM uploads WHERE id = ? AND post_id IS NULL", u1.ID).Scan(&n)
	testutil.Eq(t, n, 1)
}

func TestUpload_Serve(t *testing.T) {
	app := testutil.New(t)
	c := app.Client(t)
	img := upload(t, c, jpegBytes(t, 20, 20))
	r := c.Do("GET", img.URL, nil).Want(t, 200)
	testutil.Eq(t, r.Header.Get("Content-Type"), "image/jpeg")
	testutil.Eq(t, r.Header.Get("Cache-Control"), "public, max-age=31536000, immutable")
	testutil.Eq(t, r.Header.Get("X-Content-Type-Options"), "nosniff")
	if len(r.Body) == 0 {
		t.Fatal("empty body")
	}
	c.Do("GET", "/uploads/..%2fsecret", nil).Want(t, 404)
	c.Do("GET", "/uploads/aaaa.jpg", nil).Want(t, 404)
	c.Do("GET", "/uploads/"+img.ID+".png", nil).Want(t, 404)
}

func TestCleanupOrphans(t *testing.T) {
	app := testutil.New(t)
	c := app.Client(t)
	orphan := upload(t, c, jpegBytes(t, 10, 10))
	kept := upload(t, c, jpegBytes(t, 10, 10))
	logo := upload(t, app.AdminClient(t, "wuzi"), pngBytes(t, 10, 10))
	c.Do("POST", "/api/posts", map[string]any{"title": "Com imagem", "imageIds": []string{kept.ID}}).Want(t, 201)
	testutil.Exec(t, app.DB, "UPDATE board_settings SET logo_upload_id = ?", logo.ID)
	svc := uploads.NewService(app.DB, app.Clock, nil, nil, app.Cfg.DataDir)

	n, err := svc.CleanupOrphans(context.Background())
	if err != nil || n != 0 {
		t.Fatalf("fresh cleanup = %d %v", n, err)
	}
	app.Clock.Advance(25 * time.Hour)
	n, err = svc.CleanupOrphans(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	testutil.Eq(t, n, 1)
	if _, err := os.Stat(filePath(app, orphan)); !os.IsNotExist(err) {
		t.Fatal("orphan file not removed")
	}
	for _, img := range []api.Image{kept, logo} {
		if _, err := os.Stat(filePath(app, img)); err != nil {
			t.Fatalf("%s removed: %v", img.ID, err)
		}
	}
}

func TestAdminDeleteUpload(t *testing.T) {
	app := testutil.New(t)
	c := app.Client(t)
	img := upload(t, c, jpegBytes(t, 10, 10))
	var p api.PostDetail
	c.Do("POST", "/api/posts", map[string]any{"title": "Imagem ruim", "imageIds": []string{img.ID}}).Want(t, 201).JSON(t, &p)
	c.Do("DELETE", "/api/admin/uploads/"+img.ID, nil).Want(t, 401)
	app.AdminClient(t, "wuzi").Do("DELETE", "/api/admin/uploads/"+img.ID, nil).Want(t, 204)
	if _, err := os.Stat(filePath(app, img)); !os.IsNotExist(err) {
		t.Fatal("file not removed")
	}
	var r api.PostResponse
	c.Do("GET", fmt.Sprintf("/api/posts/%d", p.ID), nil).Want(t, 200).JSON(t, &r)
	testutil.Eq(t, len(r.Post.Images), 0)
	c.Do("GET", img.URL, nil).Want(t, 404)
}
