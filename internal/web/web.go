// Package web serves the embedded Angular build and injects link-preview meta tags.
package web

import (
	"embed"
	"html"
	"io/fs"
	"net/http"
	"path"
	"regexp"
	"strings"
)

//go:embed all:dist
var distFS embed.FS

// Dist is the embedded Angular build (internal/web/dist).
func Dist() fs.FS {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic(err)
	}
	return sub
}

// Meta is the link-preview information for one page.
type Meta struct {
	Title, Description, URL, Image string
}

// MetaSource decides the meta tags for a request path.
type MetaSource interface {
	Meta(r *http.Request) Meta
}

const placeholder = "<!-- pb:meta -->"

var hashedAsset = regexp.MustCompile(`-[A-Z0-9]{8}\.(js|css|woff2?)$`)

// fallbackIndex is served when the front-end has not been built.
const fallbackIndex = `<!doctype html><html><head><meta charset="utf-8">` + placeholder +
	`</head><body><p>PatchBoard front-end is not built. Run <code>make web</code>.</p></body></html>`

// Handler serves static files from fsys and index.html (with meta tags) for everything else.
func Handler(fsys fs.FS, src MetaSource) http.Handler {
	index := fallbackIndex
	if b, err := fs.ReadFile(fsys, "index.html"); err == nil {
		index = string(b)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if name != "" && name != "index.html" {
			if st, err := fs.Stat(fsys, name); err == nil && !st.IsDir() {
				if hashedAsset.MatchString(name) {
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				}
				http.ServeFileFS(w, r, fsys, name)
				return
			}
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		page := strings.Replace(index, placeholder, render(src.Meta(r)), 1)
		_, _ = w.Write([]byte(page))
	})
}

func render(m Meta) string {
	e := html.EscapeString
	var b strings.Builder
	b.WriteString("<title>" + e(m.Title) + "</title>")
	b.WriteString(`<meta name="description" content="` + e(m.Description) + `">`)
	b.WriteString(`<meta property="og:site_name" content="` + e(m.Title) + `">`)
	b.WriteString(`<meta property="og:type" content="website">`)
	b.WriteString(`<meta property="og:title" content="` + e(m.Title) + `">`)
	b.WriteString(`<meta property="og:description" content="` + e(m.Description) + `">`)
	if m.URL != "" {
		b.WriteString(`<meta property="og:url" content="` + e(m.URL) + `">`)
	}
	card := "summary"
	if m.Image != "" {
		b.WriteString(`<meta property="og:image" content="` + e(m.Image) + `">`)
		card = "summary_large_image"
	}
	b.WriteString(`<meta name="twitter:card" content="` + card + `">`)
	return b.String()
}
