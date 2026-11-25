package posts

import (
	"context"
	"net/http"
	"strings"
	"unicode"
	"unicode/utf8"

	"patchboard/internal/api"
	"patchboard/internal/httpx"
)

const (
	maxTokens    = 8
	similarLimit = 5
)

// FTSQuery turns free text into a safe FTS5 query: each word becomes a quoted
// prefix term. Words are ANDed for search and ORed (min 3 runes) for similar.
func FTSQuery(input string, any bool) string {
	minRunes := 1
	if any {
		minRunes = 3
	}
	words := strings.FieldsFunc(input, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsNumber(r) })
	terms := make([]string, 0, maxTokens)
	for _, w := range words {
		if utf8.RuneCountInString(w) < minRunes {
			continue
		}
		terms = append(terms, `"`+w+`"*`)
		if len(terms) == maxTokens {
			break
		}
	}
	sep := " "
	if any {
		sep = " OR "
	}
	return strings.Join(terms, sep)
}

// Similar returns up to 5 public posts whose title or body resembles title.
func (s *Service) Similar(ctx context.Context, title string) (api.Similar, error) {
	out := api.Similar{Posts: []api.SimilarPost{}}
	fts := FTSQuery(title, true)
	if fts == "" {
		return out, nil
	}
	rows, err := s.d.DB.QueryContext(ctx, `SELECT p.id, p.slug, p.title, p.vote_count, p.status_id
FROM posts_fts JOIN posts p ON p.id = posts_fts.rowid
WHERE posts_fts MATCH ? AND `+publicFilter+`
ORDER BY bm25(posts_fts) LIMIT ?`, fts, similarLimit)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var p api.SimilarPost
		if err := rows.Scan(&p.ID, &p.Slug, &p.Title, &p.VoteCount, &p.StatusID); err != nil {
			return out, err
		}
		out.Posts = append(out.Posts, p)
	}
	return out, rows.Err()
}

func (s *Service) handleSimilar(w http.ResponseWriter, r *http.Request) error {
	res, err := s.Similar(r.Context(), r.URL.Query().Get("title"))
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, res)
	return nil
}
