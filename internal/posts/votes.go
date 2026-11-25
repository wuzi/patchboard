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

// requirePublic returns 404 unless the post exists and is publicly visible.
func requirePublic(ctx context.Context, q *gen.Queries, id int64) error {
	st, err := q.GetPostState(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return httpx.ErrNotFound
	} else if err != nil {
		return err
	}
	if st.DeletedAt.Valid || st.MergedIntoID.Valid || st.Pending != 0 {
		return httpx.ErrNotFound
	}
	return nil
}

// Vote adds (up) or removes a guest's vote and returns the new state.
func (s *Service) Vote(ctx context.Context, postID int64, guestID, ipHash string, up bool) (api.VoteState, error) {
	var count int64
	err := store.InTx(ctx, s.d.DB, func(tx *sql.Tx, q *gen.Queries) error {
		if err := requirePublic(ctx, q, postID); err != nil {
			return err
		}
		var err error
		if up {
			err = q.InsertVote(ctx, gen.InsertVoteParams{PostID: postID, GuestID: guestID, IpHash: ipHash, CreatedAt: s.d.Clock.Now().Unix()})
		} else {
			err = q.DeleteVote(ctx, gen.DeleteVoteParams{PostID: postID, GuestID: guestID})
		}
		if err != nil {
			return err
		}
		count, err = q.RecountVotes(ctx, postID)
		return err
	})
	return api.VoteState{VoteCount: count, Voted: up}, err
}

func (s *Service) handleVote(up bool) httpx.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		id, err := postID(r)
		if err != nil {
			return err
		}
		ctx := r.Context()
		if err := requirePublic(ctx, s.q, id); err != nil {
			return err
		}
		if err := s.d.Auth.Limit(r, httpx.RuleVote); err != nil {
			return err
		}
		g, err := s.d.Auth.GuestWriter(w, r)
		if err != nil {
			return err
		}
		v, err := s.Vote(ctx, id, g.ID, auth.ActorFrom(ctx).Info.IPHash, up)
		if err != nil {
			return err
		}
		httpx.WriteJSON(w, http.StatusOK, v)
		return nil
	}
}
