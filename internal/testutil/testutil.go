// Package testutil runs the real server against a fresh SQLite database for API tests.
package testutil

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"testing/fstest"
	"time"

	"patchboard/internal/api"
	"patchboard/internal/auth"
	"patchboard/internal/clock"
	"patchboard/internal/config"
	"patchboard/internal/server"
	"patchboard/internal/store"
)

// IndexHTML is the SPA shell served in tests.
const IndexHTML = "<html><head><!-- pb:meta --></head><body></body></html>"

type App struct {
	URL   string
	DB    *sql.DB
	Clock *clock.Fake
	Cfg   config.Config

	nextIP atomic.Int32
}

// New starts a server with a fresh database, a fake clock at 2026-11-25T12:00:00Z,
// 127.0.0.0/8 as trusted proxy (so clients can pick their IP) and base URL http://pb.test.
func New(t *testing.T, opts ...func(*config.Config)) *App {
	t.Helper()
	dir := t.TempDir()
	cfg := config.Config{
		Addr:           ":0",
		DataDir:        dir,
		BaseURL:        &url.URL{Scheme: "http", Host: "pb.test"},
		TrustedProxies: []netip.Prefix{netip.MustParsePrefix("127.0.0.0/8")},
		Secret:         []byte("test-secret-test-secret-test-sec"),
	}
	for _, o := range opts {
		o(&cfg)
	}
	db, err := store.Open(filepath.Join(dir, "patchboard.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := store.Migrate(context.Background(), db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	fake := clock.NewFake(time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC))
	h, err := server.New(server.Deps{
		Cfg: cfg, DB: db, Clock: fake,
		WebFS: fstest.MapFS{"index.html": {Data: []byte(IndexHTML)}},
	})
	if err != nil {
		t.Fatalf("server: %v", err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return &App{URL: srv.URL, DB: db, Clock: fake, Cfg: cfg}
}

type Client struct {
	app    *App
	http   *http.Client
	jar    http.CookieJar
	IP     string
	noCSRF bool
}

// Client returns a client with its own cookie jar and a unique IP 198.51.100.<n>.
func (a *App) Client(t *testing.T) *Client {
	t.Helper()
	jar, _ := cookiejar.New(nil)
	n := a.nextIP.Add(1)
	c := &Client{app: a, jar: jar, IP: fmt.Sprintf("198.51.100.%d", n%250+1)}
	if n >= 250 {
		c.IP = fmt.Sprintf("198.18.%d.%d", n/250, n%250+1)
	}
	c.http = &http.Client{
		Jar:           jar,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	return c
}

// WithIP returns a copy sharing the cookie jar but sending another X-Forwarded-For.
func (c *Client) WithIP(ip string) *Client {
	cp := *c
	cp.IP = ip
	return &cp
}

// NoCSRF returns a copy that omits the X-PB-CSRF header.
func (c *Client) NoCSRF() *Client {
	cp := *c
	cp.noCSRF = true
	return &cp
}

type Resp struct {
	Code   int
	Body   []byte
	Header http.Header
}

// Do sends body as JSON (nil = no body).
func (c *Client) Do(method, path string, body any) *Resp {
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			panic(err)
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.app.URL+path, rd)
	if err != nil {
		panic(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.send(req)
}

// Upload sends a multipart form with one file field.
func (c *Client) Upload(path, field, filename string, data []byte) *Resp {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile(field, filename)
	fw.Write(data)
	mw.Close()
	req, _ := http.NewRequest("POST", c.app.URL+path, &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return c.send(req)
}

// Send sends a prepared request with the client's IP, CSRF header and cookies.
func (c *Client) Send(req *http.Request) *Resp { return c.send(req) }

func (c *Client) send(req *http.Request) *Resp {
	if !c.noCSRF {
		req.Header.Set("X-PB-CSRF", "1")
	}
	req.Header.Set("X-Forwarded-For", c.IP)
	res, err := c.http.Do(req)
	if err != nil {
		panic(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return &Resp{Code: res.StatusCode, Body: b, Header: res.Header}
}

// Cookie returns the named cookie from the jar, or nil.
func (c *Client) Cookie(name string) *http.Cookie {
	u, _ := url.Parse(c.app.URL)
	for _, ck := range c.jar.Cookies(u) {
		if ck.Name == name {
			return ck
		}
	}
	return nil
}

// Want fails the test unless the status matches.
func (r *Resp) Want(t *testing.T, status int) *Resp {
	t.Helper()
	if r.Code != status {
		t.Fatalf("status = %d, want %d; body: %s", r.Code, status, r.Body)
	}
	return r
}

// JSON decodes the body into v.
func (r *Resp) JSON(t *testing.T, v any) {
	t.Helper()
	if err := json.Unmarshal(r.Body, v); err != nil {
		t.Fatalf("decode %s: %v", r.Body, err)
	}
}

type errBody struct {
	Error struct {
		Code   string            `json:"code"`
		Fields map[string]string `json:"fields"`
	} `json:"error"`
}

// ErrCode returns error.code from the body.
func (r *Resp) ErrCode(t *testing.T) string {
	t.Helper()
	var e errBody
	r.JSON(t, &e)
	return e.Error.Code
}

// Field returns error.fields[name] from the body.
func (r *Resp) Field(t *testing.T, name string) string {
	t.Helper()
	var e errBody
	r.JSON(t, &e)
	return e.Error.Fields[name]
}

// SetCookie finds a Set-Cookie header by name.
func (r *Resp) SetCookie(name string) *http.Cookie {
	for _, ck := range (&http.Response{Header: r.Header}).Cookies() {
		if ck.Name == name {
			return ck
		}
	}
	return nil
}

// Eq fails unless got == want.
func Eq[T comparable](t *testing.T, got, want T) {
	t.Helper()
	if got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// Exec runs raw SQL for test setup.
func Exec(t *testing.T, db *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := db.Exec(query, args...); err != nil {
		t.Fatalf("exec %q: %v", query, err)
	}
}

// AdminPassword is the password AdminClient gives every admin it creates.
const AdminPassword = "correct-horse-1"

// AdminClient creates an admin (display name = username title-cased) and returns a logged-in client.
func (a *App) AdminClient(t *testing.T, username string) *Client {
	t.Helper()
	display := strings.ToUpper(username[:1]) + username[1:]
	if _, err := auth.CreateAdmin(context.Background(), a.DB, a.Clock.Now(), username, display, AdminPassword); err != nil {
		t.Fatalf("create admin: %v", err)
	}
	c := a.Client(t)
	c.Do("POST", "/api/admin/login", map[string]string{"username": username, "password": AdminPassword}).Want(t, 200)
	return c
}

// InsertPost inserts a post directly (authored by a seed admin) and returns its id.
func InsertPost(t *testing.T, db *sql.DB, title string, statusID int64) int64 {
	t.Helper()
	Exec(t, db, `INSERT OR IGNORE INTO admins (id, username, display_name, password_hash, created_at) VALUES (1000, 'seed', 'Seed', 'x', 0)`)
	res, err := db.Exec(`INSERT INTO posts (slug, title, status_id, author_admin_id, ip_hash, created_at, updated_at)
		VALUES ('post', ?, ?, 1000, 'seed', 0, 0)`, title, statusID)
	if err != nil {
		t.Fatalf("insert post: %v", err)
	}
	id, _ := res.LastInsertId()
	return id
}

// NewPost creates a post through the API as this client.
func (c *Client) NewPost(t *testing.T, title string) api.PostDetail {
	t.Helper()
	var p api.PostDetail
	c.Do("POST", "/api/posts", map[string]any{"title": title}).Want(t, 201).JSON(t, &p)
	return p
}
