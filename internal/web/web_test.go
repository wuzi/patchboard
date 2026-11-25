package web_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"patchboard/internal/web"
)

type fixedMeta web.Meta

func (f fixedMeta) Meta(*http.Request) web.Meta { return web.Meta(f) }

var site = fstest.MapFS{
	"index.html":          {Data: []byte("<html><head><!-- pb:meta --></head><body>app</body></html>")},
	"main-ABCD1234.js":    {Data: []byte("console.log(1)")},
	"styles-XYZ98765.css": {Data: []byte("body{}")},
	"favicon.svg":         {Data: []byte("<svg/>")},
}

func get(t *testing.T, h http.Handler, path string) (*http.Response, string) {
	t.Helper()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
	res := w.Result()
	b, _ := io.ReadAll(res.Body)
	return res, string(b)
}

func TestWeb_StaticAndFallback(t *testing.T) {
	h := web.Handler(site, fixedMeta{Title: "Venore", Description: "Board", URL: "http://pb.test/roadmap"})
	res, body := get(t, h, "/main-ABCD1234.js")
	if body != "console.log(1)" || res.Header.Get("Cache-Control") != "public, max-age=31536000, immutable" {
		t.Fatalf("js: %q %q", body, res.Header.Get("Cache-Control"))
	}
	res, _ = get(t, h, "/favicon.svg")
	if res.Header.Get("Cache-Control") == "public, max-age=31536000, immutable" {
		t.Fatal("unhashed files must not be immutable")
	}
	res, body = get(t, h, "/roadmap")
	if !strings.Contains(body, "<title>Venore</title>") || !strings.Contains(body, `<meta property="og:url" content="http://pb.test/roadmap">`) {
		t.Fatalf("index body = %s", body)
	}
	if strings.Contains(body, "pb:meta") {
		t.Fatal("placeholder not replaced")
	}
	if res.Header.Get("Cache-Control") != "no-cache" || !strings.HasPrefix(res.Header.Get("Content-Type"), "text/html") {
		t.Fatalf("index headers = %v", res.Header)
	}
	_, body = get(t, h, "/favicon.ico")
	if !strings.Contains(body, "app") {
		t.Fatal("missing file should fall back to the SPA")
	}
	_, body = get(t, h, "/index.html")
	if !strings.Contains(body, "<title>Venore</title>") {
		t.Fatal("/index.html must be the injected page")
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("POST", "/roadmap", nil))
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST status = %d", w.Code)
	}
}

func TestWeb_EscapesMeta(t *testing.T) {
	h := web.Handler(site, fixedMeta{Title: `<script>"x"</script>`, Description: "a & b", Image: "http://pb.test/uploads/x.png"})
	_, body := get(t, h, "/")
	if strings.Contains(body, `<script>"x"`) {
		t.Fatalf("unescaped title: %s", body)
	}
	if !strings.Contains(body, "&lt;script&gt;&#34;x&#34;&lt;/script&gt;") || !strings.Contains(body, "a &amp; b") {
		t.Fatalf("escaped values missing: %s", body)
	}
	if !strings.Contains(body, `<meta name="twitter:card" content="summary_large_image">`) {
		t.Fatal("image should give a large card")
	}
	_, body = get(t, web.Handler(site, fixedMeta{Title: "t"}), "/")
	if strings.Contains(body, "og:image") || !strings.Contains(body, `content="summary"`) {
		t.Fatalf("no-image card wrong: %s", body)
	}
}

func TestWeb_MissingIndex(t *testing.T) {
	_, body := get(t, web.Handler(fstest.MapFS{}, fixedMeta{Title: "t"}), "/")
	if !strings.Contains(body, "<title>t</title>") {
		t.Fatalf("fallback page = %s", body)
	}
}
