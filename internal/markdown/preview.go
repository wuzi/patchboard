package markdown

import (
	"net/http"
	"unicode/utf8"

	"patchboard/internal/httpx"
)

// PreviewHandler renders Markdown exactly as posts will (POST /api/markdown/preview).
func PreviewHandler(lim *httpx.Limiter) httpx.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		if ok, retry := lim.Allow(httpx.RulePreview, httpx.InfoFrom(r.Context()).IPHash); !ok {
			return httpx.RateLimited(retry)
		}
		var req struct {
			Markdown string `json:"markdown"`
		}
		if err := httpx.DecodeJSON(r, &req); err != nil {
			return err
		}
		if utf8.RuneCountInString(req.Markdown) > MaxBody {
			return httpx.Validation().Field("markdown", "too_long")
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]string{"html": Render(req.Markdown)})
		return nil
	}
}
