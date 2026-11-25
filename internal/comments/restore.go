package comments

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"time"

	"patchboard/internal/api"
	"patchboard/internal/httpx"
	"patchboard/internal/markdown"
	"patchboard/internal/store"
	"patchboard/internal/store/gen"
)

// Restore undeletes a comment and recounts its post.
func (s *Service) Restore(ctx context.Context, id int64) error {
	c, err := s.q.GetComment(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return httpx.ErrNotFound
	} else if err != nil {
		return err
	}
	return store.InTx(ctx, s.db, func(tx *sql.Tx, q *gen.Queries) error {
		if err := q.RestoreComment(ctx, id); err != nil {
			return err
		}
		return q.RecountComments(ctx, c.PostID)
	})
}

// Trash lists soft-deleted comments, most recently deleted first.
func (s *Service) Trash(ctx context.Context, limit int) ([]api.TrashComment, error) {
	rows, err := s.q.TrashComments(ctx, int64(limit))
	if err != nil {
		return nil, err
	}
	out := make([]api.TrashComment, 0, len(rows))
	for _, c := range rows {
		by := "author"
		if c.DeletedByAdminID.Valid {
			by = "admin"
		}
		out = append(out, api.TrashComment{
			ID: c.ID, PostID: c.PostID, PostTitle: c.PostTitle, Excerpt: markdown.Excerpt(c.BodyMd, 140),
			DeletedAt: time.Unix(c.DeletedAt.Int64, 0).UTC(), DeletedBy: by,
		})
	}
	return out, nil
}

func (s *Service) handleRestore(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	if err := s.Restore(r.Context(), id); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}
