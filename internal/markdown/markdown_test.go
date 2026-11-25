package markdown_test

import (
	"strings"
	"testing"
	"unicode/utf8"

	"patchboard/internal/markdown"
	"patchboard/internal/testutil"
)

func TestRender(t *testing.T) {
	cases := []struct {
		in          string
		contains    []string
		notContains []string
	}{
		{"**bold**", []string{"<strong>bold</strong>"}, nil},
		{"<script>alert(1)</script>", nil, []string{"<script"}},
		{"<b>raw</b>", nil, []string{"<b>"}},
		{"[x](javascript:alert(1))", nil, []string{"javascript:"}},
		{"![a](https://e.com/a.png)", nil, []string{"<img"}},
		{"see https://example.com", []string{`href="https://example.com"`, "nofollow", `target="_blank"`}, nil},
		{"# Title", []string{"<h1"}, nil},
		{"| a | b |\n|---|---|\n| 1 | 2 |", []string{"<table>"}, nil},
		{"~~old~~", []string{"<del>"}, nil},
		{"- [x] done", nil, []string{"<input"}},
	}
	for _, c := range cases {
		out := markdown.Render(c.in)
		for _, s := range c.contains {
			if !strings.Contains(out, s) {
				t.Errorf("Render(%q) = %q, missing %q", c.in, out, s)
			}
		}
		for _, s := range c.notContains {
			if strings.Contains(out, s) {
				t.Errorf("Render(%q) = %q, must not contain %q", c.in, out, s)
			}
		}
	}
}

func TestExcerpt(t *testing.T) {
	testutil.Eq(t, markdown.Excerpt("Na **\"Loja\"** do Xodet\n\n- item", 140), `Na "Loja" do Xodet item`)
	testutil.Eq(t, markdown.Excerpt("", 140), "")
	long := strings.Repeat("palavra ", 40) // 320 runes
	got := markdown.Excerpt(long, 140)
	if n := utf8.RuneCountInString(got); n > 141 {
		t.Fatalf("excerpt has %d runes", n)
	}
	if !strings.HasSuffix(got, "…") || strings.HasSuffix(got, " …") || strings.Contains(got, "palavr…") {
		t.Fatalf("excerpt not cut at a word boundary: %q", got)
	}
}

func TestPreviewHandler(t *testing.T) {
	app := testutil.New(t)
	c := app.Client(t)
	var out struct{ HTML string }
	c.Do("POST", "/api/markdown/preview", map[string]string{"markdown": "**a**"}).Want(t, 200).JSON(t, &out)
	if !strings.Contains(out.HTML, "<strong>a</strong>") {
		t.Fatalf("html = %q", out.HTML)
	}
	r := c.Do("POST", "/api/markdown/preview", map[string]string{"markdown": strings.Repeat("a", 10001)}).Want(t, 422)
	testutil.Eq(t, r.Field(t, "markdown"), "too_long")
	lim := app.Client(t)
	for i := 0; i < 120; i++ {
		lim.Do("POST", "/api/markdown/preview", map[string]string{"markdown": "x"}).Want(t, 200)
	}
	testutil.Eq(t, lim.Do("POST", "/api/markdown/preview", map[string]string{"markdown": "x"}).Want(t, 429).ErrCode(t), "rate_limited")
}
