// Package markdown renders user Markdown to sanitized HTML.
package markdown

import (
	"bytes"
	"html"
	"strings"
	"unicode"

	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

// MaxBody is the longest Markdown accepted for posts, Updates and previews.
const MaxBody = 10000

var (
	md = goldmark.New(goldmark.WithExtensions(
		extension.Table, extension.Strikethrough, extension.Linkify,
	)) // no html.WithUnsafe(): raw HTML is dropped by the renderer

	policy = func() *bluemonday.Policy {
		p := bluemonday.NewPolicy()
		p.AllowElements("p", "br", "strong", "em", "del", "code", "pre", "blockquote",
			"ul", "ol", "li", "h1", "h2", "h3", "h4", "h5", "h6",
			"table", "thead", "tbody", "tr", "th", "td", "hr", "a")
		p.AllowAttrs("href").OnElements("a")
		p.AllowAttrs("align").OnElements("th", "td")
		p.AllowStandardURLs()
		p.RequireNoFollowOnLinks(true)
		p.AddTargetBlankToFullyQualifiedLinks(true)
		return p
	}()

	strict = bluemonday.StrictPolicy()
)

// Render converts Markdown to sanitized HTML.
func Render(src string) string {
	var buf bytes.Buffer
	if err := md.Convert([]byte(src), &buf); err != nil {
		return ""
	}
	return policy.Sanitize(buf.String())
}

// Excerpt returns up to max runes of plain text, cut on a word boundary with "…".
func Excerpt(src string, max int) string {
	text := html.UnescapeString(strict.Sanitize(Render(src)))
	text = strings.Join(strings.FieldsFunc(text, unicode.IsSpace), " ")
	runes := []rune(text)
	if len(runes) <= max {
		return text
	}
	cut := string(runes[:max])
	if i := strings.LastIndexFunc(cut, unicode.IsSpace); i > 0 && !unicode.IsSpace(runes[max]) {
		cut = cut[:i]
	}
	return strings.TrimRightFunc(cut, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsPunct(r) }) + "…"
}
