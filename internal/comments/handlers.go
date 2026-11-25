package comments

import (
	"net/http"
	"strconv"

	"patchboard/internal/httpx"
)

// Register adds the comment routes.
func (s *Service) Register(mux *http.ServeMux, requireAdmin func(http.Handler) http.Handler) {
	mux.Handle("POST /api/admin/comments/{id}/restore", requireAdmin(httpx.HandlerFunc(s.handleRestore)))
	mux.Handle("POST /api/posts/{id}/comments", httpx.HandlerFunc(s.handleCreate))
	mux.Handle("PATCH /api/comments/{id}", httpx.HandlerFunc(s.handleUpdate))
	mux.Handle("DELETE /api/comments/{id}", httpx.HandlerFunc(s.handleDelete))
	mux.Handle("POST /api/comments/{id}/like", s.handleLike(true))
	mux.Handle("DELETE /api/comments/{id}/like", s.handleLike(false))
}

func pathID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, httpx.ErrNotFound
	}
	return id, nil
}
