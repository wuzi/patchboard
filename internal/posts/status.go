package posts

import (
	"context"
	"database/sql"
	"net/http"
	"strings"
	"unicode/utf8"

	"patchboard/internal/api"
	"patchboard/internal/auth"
	"patchboard/internal/httpx"
	"patchboard/internal/markdown"
	"patchboard/internal/store"
	"patchboard/internal/store/gen"
)

const maxResponse = 5000

// ChangeStatus moves a post, optionally sets/clears the team response and publishes an Update, atomically.
func (s *Service) ChangeStatus(ctx context.Context, admin *auth.Admin, postID int64, req api.ChangeStatusRequest) (api.PostDetail, error) {
	p, err := s.load(ctx, postID)
	if err != nil {
		return api.PostDetail{}, err
	}
	if p.DeletedAt.Valid || p.MergedIntoID.Valid {
		return api.PostDetail{}, httpx.ErrNotFound
	}
	e := httpx.Validation()
	if ok, err := s.d.Board.StatusExists(ctx, req.StatusID); err != nil {
		return api.PostDetail{}, err
	} else if !ok {
		e.Field("statusId", "invalid")
	}
	if req.Response != nil {
		trimmed := strings.TrimSpace(*req.Response)
		req.Response = &trimmed
		if utf8.RuneCountInString(trimmed) > maxResponse {
			e.Field("response", "too_long")
		}
	}
	if len(e.Fields) > 0 {
		return api.PostDetail{}, e
	}
	now := s.d.Clock.Now().Unix()
	err = store.InTx(ctx, s.d.DB, func(tx *sql.Tx, q *gen.Queries) error {
		if err := q.SetPostStatus(ctx, gen.SetPostStatusParams{StatusID: req.StatusID, UpdatedAt: now, ID: postID}); err != nil {
			return err
		}
		if req.Response != nil {
			params := gen.SetPostResponseParams{ID: postID}
			if *req.Response != "" {
				params.ResponseMd = sql.NullString{String: *req.Response, Valid: true}
				params.ResponseHtml = sql.NullString{String: markdown.Render(*req.Response), Valid: true}
				params.ResponseAdminID = sql.NullInt64{Int64: admin.ID, Valid: true}
				params.ResponseAt = sql.NullInt64{Int64: now, Valid: true}
			}
			if err := q.SetPostResponse(ctx, params); err != nil {
				return err
			}
		}
		if req.Update != nil {
			_, err := s.d.CreateUpdate(ctx, tx, admin.ID, req.Update.Title, req.Update.Body, []int64{postID})
			return err
		}
		return nil
	})
	if err != nil {
		return api.PostDetail{}, err
	}
	res, err := s.Detail(ctx, postID, &auth.Actor{Admin: admin})
	if err != nil {
		return api.PostDetail{}, err
	}
	return *res.Post, nil
}

func (s *Service) handleChangeStatus(w http.ResponseWriter, r *http.Request) error {
	id, err := postID(r)
	if err != nil {
		return err
	}
	var req api.ChangeStatusRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	viewer := auth.ActorFrom(r.Context())
	d, err := s.ChangeStatus(r.Context(), viewer.Admin, id, req)
	if err != nil {
		return err
	}
	res, err := s.Detail(r.Context(), d.ID, viewer)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, res.Post)
	return nil
}
