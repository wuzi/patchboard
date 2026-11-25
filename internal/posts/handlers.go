package posts

import (
	"net/http"
	"strconv"

	"patchboard/internal/httpx"
)

// Register adds the post routes.
func (s *Service) Register(mux *http.ServeMux, requireAdmin func(http.Handler) http.Handler) {
	mux.Handle("POST /api/posts", httpx.HandlerFunc(s.handleCreate))
	mux.Handle("GET /api/posts/{id}", httpx.HandlerFunc(s.handleGet))
	mux.Handle("PATCH /api/posts/{id}", httpx.HandlerFunc(s.handleUpdate))
	mux.Handle("DELETE /api/posts/{id}", httpx.HandlerFunc(s.handleDelete))
}

func postID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, httpx.ErrNotFound
	}
	return id, nil
}
