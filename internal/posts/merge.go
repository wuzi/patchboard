package posts

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	"patchboard/internal/api"
	"patchboard/internal/auth"
	"patchboard/internal/httpx"
	"patchboard/internal/store"
	"patchboard/internal/store/gen"
)

func invalidMerge(msg string) *httpx.Error {
	return httpx.E(http.StatusUnprocessableEntity, "invalid_merge", msg)
}

// Merge folds post id into intoID: votes (deduplicated) and comments move, and
// anything already merged into id now points at intoID.
func (s *Service) Merge(ctx context.Context, admin *auth.Admin, id, intoID int64) (api.PostDetail, error) {
	if id == intoID {
		return api.PostDetail{}, invalidMerge("a post cannot be merged into itself")
	}
	err := store.InTx(ctx, s.d.DB, func(tx *sql.Tx, q *gen.Queries) error {
		src, err := q.GetPostFull(ctx, id)
		if errors.Is(err, sql.ErrNoRows) {
			return httpx.ErrNotFound
		} else if err != nil {
			return err
		}
		dst, err := q.GetPostFull(ctx, intoID)
		if errors.Is(err, sql.ErrNoRows) {
			return invalidMerge("target post does not exist")
		} else if err != nil {
			return err
		}
		if src.DeletedAt.Valid || dst.DeletedAt.Valid || src.MergedIntoID.Valid || dst.MergedIntoID.Valid {
			return invalidMerge("both posts must be live and not already merged")
		}
		if err := q.MoveVotes(ctx, gen.MoveVotesParams{IntoID: intoID, FromID: id}); err != nil {
			return err
		}
		if err := q.DeletePostVotes(ctx, id); err != nil {
			return err
		}
		if err := q.MoveComments(ctx, gen.MoveCommentsParams{IntoID: intoID, FromID: id}); err != nil {
			return err
		}
		if err := q.SetMergedInto(ctx, gen.SetMergedIntoParams{IntoID: sql.NullInt64{Int64: intoID, Valid: true}, FromID: id}); err != nil {
			return err
		}
		if err := q.RepointMerged(ctx, gen.RepointMergedParams{IntoID: sql.NullInt64{Int64: intoID, Valid: true}, FromID: sql.NullInt64{Int64: id, Valid: true}}); err != nil {
			return err
		}
		for _, pid := range []int64{id, intoID} {
			if _, err := q.RecountVotes(ctx, pid); err != nil {
				return err
			}
			if err := q.RecountComments(ctx, pid); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return api.PostDetail{}, err
	}
	res, err := s.Detail(ctx, intoID, &auth.Actor{Admin: admin})
	if err != nil {
		return api.PostDetail{}, err
	}
	return *res.Post, nil
}

func (s *Service) handleMerge(w http.ResponseWriter, r *http.Request) error {
	id, err := postID(r)
	if err != nil {
		return err
	}
	var req struct {
		IntoID int64 `json:"intoId"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	if _, err := s.Merge(r.Context(), auth.ActorFrom(r.Context()).Admin, id, req.IntoID); err != nil {
		return err
	}
	return s.writeDetail(w, r, req.IntoID)
}
