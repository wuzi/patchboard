// Package posts owns posts, votes, listing, search and moderation.
package posts

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

const maxSlug = 60

// Slugify turns a title into an ASCII URL slug ("Ação rápida" → "acao-rapida").
func Slugify(title string) string {
	var b strings.Builder
	dash := false
	for _, r := range norm.NFD.String(strings.ToLower(title)) {
		switch {
		case unicode.Is(unicode.Mn, r):
			continue
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			if dash && b.Len() > 0 {
				b.WriteByte('-')
			}
			dash = false
			b.WriteRune(r)
		default:
			dash = true
		}
	}
	s := b.String()
	if len(s) > maxSlug {
		cut := s[:maxSlug]
		if i := strings.LastIndexByte(cut, '-'); i > 0 && s[maxSlug] != '-' {
			cut = cut[:i]
		}
		s = strings.Trim(cut, "-")
	}
	if s == "" {
		return "post"
	}
	return s
}
