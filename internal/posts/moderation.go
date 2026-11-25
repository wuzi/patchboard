package posts

import (
	"context"
	"database/sql"
	"net/http"

	"patchboard/internal/api"
	"patchboard/internal/auth"
	"patchboard/internal/httpx"
	"patchboard/internal/store/gen"
)

// live loads a post that is neither deleted nor merged (404 otherwise).
func (s *Service) live(ctx context.Context, id int64) (postRow, error) {
	p, err := s.load(ctx, id)
	if err != nil {
		return p, err
	}
	if p.DeletedAt.Valid || p.MergedIntoID.Valid {
		return p, httpx.ErrNotFound
	}
	return p, nil
}

// Pending lists posts awaiting approval, oldest first.
func (s *Service) Pending(ctx context.Context, viewer *auth.Actor) ([]api.PostCard, error) {
	f := filter{}
	f.add("p.deleted_at IS NULL AND p.merged_into_id IS NULL AND p.pending = 1")
	return s.queryCards(ctx, viewer, f, "p.created_at, p.id", 500, 0)
}

func (s *Service) writeDetail(w http.ResponseWriter, r *http.Request, id int64) error {
	res, err := s.Detail(r.Context(), id, auth.ActorFrom(r.Context()))
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, res.Post)
	return nil
}

func (s *Service) handlePending(w http.ResponseWriter, r *http.Request) error {
	cards, err := s.Pending(r.Context(), auth.ActorFrom(r.Context()))
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, cards)
	return nil
}

func (s *Service) handleApprove(w http.ResponseWriter, r *http.Request) error {
	id, err := postID(r)
	if err != nil {
		return err
	}
	if _, err := s.live(r.Context(), id); err != nil {
		return err
	}
	if err := s.q.ApprovePost(r.Context(), id); err != nil {
		return err
	}
	return s.writeDetail(w, r, id)
}

func (s *Service) handlePin(w http.ResponseWriter, r *http.Request) error {
	id, err := postID(r)
	if err != nil {
		return err
	}
	var req struct {
		Pinned bool `json:"pinned"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	if _, err := s.live(r.Context(), id); err != nil {
		return err
	}
	at := sql.NullInt64{}
	if req.Pinned {
		at = sql.NullInt64{Int64: s.d.Clock.Now().Unix(), Valid: true}
	}
	if err := s.q.SetPinned(r.Context(), gen.SetPinnedParams{PinnedAt: at, ID: id}); err != nil {
		return err
	}
	return s.writeDetail(w, r, id)
}

func (s *Service) handleLock(w http.ResponseWriter, r *http.Request) error {
	id, err := postID(r)
	if err != nil {
		return err
	}
	var req struct {
		Locked bool `json:"locked"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	if _, err := s.live(r.Context(), id); err != nil {
		return err
	}
	locked := int64(0)
	if req.Locked {
		locked = 1
	}
	if err := s.q.SetLocked(r.Context(), gen.SetLockedParams{CommentsLocked: locked, ID: id}); err != nil {
		return err
	}
	return s.writeDetail(w, r, id)
}

func (s *Service) handleRestore(w http.ResponseWriter, r *http.Request) error {
	id, err := postID(r)
	if err != nil {
		return err
	}
	if _, err := s.load(r.Context(), id); err != nil {
		return err
	}
	if err := s.q.RestorePost(r.Context(), id); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// TrashPosts lists soft-deleted posts, most recently deleted first.
func (s *Service) TrashPosts(ctx context.Context, limit int) ([]api.TrashPost, error) {
	rows, err := s.q.TrashPosts(ctx, int64(limit))
	if err != nil {
		return nil, err
	}
	out := make([]api.TrashPost, 0, len(rows))
	for _, p := range rows {
		by := "author"
		if p.DeletedByAdminID.Valid {
			by = "admin"
		}
		out = append(out, api.TrashPost{ID: p.ID, Slug: p.Slug, Title: p.Title, DeletedAt: ts(p.DeletedAt.Int64), DeletedBy: by})
	}
	return out, nil
}
