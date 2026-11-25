package server

import (
	"net/http"

	"patchboard/internal/api"
	"patchboard/internal/comments"
	"patchboard/internal/httpx"
	"patchboard/internal/posts"
)

const trashLimit = 100

// trashHandler serves GET /api/admin/trash (deleted posts and comments).
func trashHandler(p *posts.Service, c *comments.Service) httpx.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		var t api.Trash
		var err error
		if t.Posts, err = p.TrashPosts(r.Context(), trashLimit); err != nil {
			return err
		}
		if t.Comments, err = c.Trash(r.Context(), trashLimit); err != nil {
			return err
		}
		httpx.WriteJSON(w, http.StatusOK, t)
		return nil
	}
}
