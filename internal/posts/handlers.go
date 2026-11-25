package posts

import (
	"net/http"
	"strconv"

	"patchboard/internal/httpx"
)

// Register adds the post routes.
func (s *Service) Register(mux *http.ServeMux, requireAdmin func(http.Handler) http.Handler) {
	mux.Handle("POST /api/posts", httpx.HandlerFunc(s.handleCreate))
	mux.Handle("GET /api/posts", httpx.HandlerFunc(s.handleList))
	mux.Handle("GET /api/roadmap", httpx.HandlerFunc(s.handleRoadmap))
	mux.Handle("GET /api/posts/similar", httpx.HandlerFunc(s.handleSimilar))
	mux.Handle("GET /api/posts/{id}", httpx.HandlerFunc(s.handleGet))
	mux.Handle("PATCH /api/posts/{id}", httpx.HandlerFunc(s.handleUpdate))
	mux.Handle("DELETE /api/posts/{id}", httpx.HandlerFunc(s.handleDelete))
	mux.Handle("POST /api/posts/{id}/vote", s.handleVote(true))
	mux.Handle("DELETE /api/posts/{id}/vote", s.handleVote(false))
}

func postID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, httpx.ErrNotFound
	}
	return id, nil
}
