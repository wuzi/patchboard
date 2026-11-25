package comments

import (
	"database/sql"
	"errors"
	"net/http"

	"patchboard/internal/api"
	"patchboard/internal/auth"
	"patchboard/internal/httpx"
	"patchboard/internal/store"
	"patchboard/internal/store/gen"
)

func (s *Service) handleLike(up bool) httpx.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		id, err := pathID(r)
		if err != nil {
			return err
		}
		ctx := r.Context()
		c, err := s.q.GetComment(ctx, id)
		if errors.Is(err, sql.ErrNoRows) || (err == nil && c.DeletedAt.Valid) {
			return httpx.ErrNotFound
		} else if err != nil {
			return err
		}
		if err := s.visiblePost(ctx, c.PostID, auth.ActorFrom(ctx)); err != nil {
			return err
		}
		if err := s.auth.Limit(r, httpx.RuleLike); err != nil {
			return err
		}
		g, err := s.auth.GuestWriter(w, r)
		if err != nil {
			return err
		}
		var count int64
		err = store.InTx(ctx, s.db, func(tx *sql.Tx, q *gen.Queries) error {
			if up {
				err = q.InsertLike(ctx, gen.InsertLikeParams{CommentID: id, GuestID: g.ID, CreatedAt: s.clock.Now().Unix()})
			} else {
				err = q.DeleteLike(ctx, gen.DeleteLikeParams{CommentID: id, GuestID: g.ID})
			}
			if err != nil {
				return err
			}
			count, err = q.RecountLikes(ctx, id)
			return err
		})
		if err != nil {
			return err
		}
		httpx.WriteJSON(w, http.StatusOK, api.LikeState{LikeCount: count, Liked: up})
		return nil
	}
}
