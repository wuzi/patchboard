package server

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"patchboard/internal/auth"
	"patchboard/internal/board"
	"patchboard/internal/config"
	"patchboard/internal/posts"
	"patchboard/internal/web"
)

var postPath = regexp.MustCompile(`^/p/(\d+)(?:-[^/]*)?/?$`)

// metaSource builds link-preview tags: a public post's own, otherwise the board's.
type metaSource struct {
	cfg   config.Config
	board *board.Service
	posts *posts.Service
}

func (m metaSource) abs(p string) string { return m.cfg.BaseURL.String() + p }

func prefersEnglish(r *http.Request) bool {
	first := strings.TrimSpace(strings.Split(r.Header.Get("Accept-Language"), ",")[0])
	first = strings.ToLower(strings.SplitN(strings.SplitN(first, ";", 2)[0], "-", 2)[0])
	return first == "en"
}

func (m metaSource) Meta(r *http.Request) web.Meta {
	ctx := r.Context()
	b, err := m.board.Get(ctx)
	if err != nil {
		return web.Meta{Title: "PatchBoard"}
	}
	meta := web.Meta{Title: b.Title.PT, Description: b.Description.PT, URL: m.abs(r.URL.Path)}
	if prefersEnglish(r) {
		meta.Title, meta.Description = b.Title.EN, b.Description.EN
	}
	if b.LogoURL != nil {
		meta.Image = m.abs(*b.LogoURL)
	}
	match := postPath.FindStringSubmatch(r.URL.Path)
	if match == nil {
		return meta
	}
	id, _ := strconv.ParseInt(match[1], 10, 64)
	res, err := m.posts.Detail(ctx, id, &auth.Actor{}) // anonymous viewer: hidden posts 404
	if err != nil || res.Post == nil {
		return meta
	}
	p := res.Post
	meta.Title = p.Title
	meta.Description = p.Excerpt
	meta.URL = m.abs("/p/" + strconv.FormatInt(p.ID, 10) + "-" + p.Slug)
	if len(p.Images) > 0 {
		meta.Image = m.abs(p.Images[0].URL)
	}
	return meta
}
