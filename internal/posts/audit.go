package posts

import (
	"database/sql"
	"net/http"
	"time"

	"patchboard/internal/api"
	"patchboard/internal/httpx"
	"patchboard/internal/store"
	"patchboard/internal/store/gen"
)

const (
	sharedIPThreshold = 1                // at least one other voter on the same IP hash
	newGuestWindow    = 10 * time.Minute // identity created this soon before voting
)

func (s *Service) handleVoteAudit(w http.ResponseWriter, r *http.Request) error {
	id, err := postID(r)
	if err != nil {
		return err
	}
	if _, err := s.load(r.Context(), id); err != nil {
		return err
	}
	rows, err := s.q.ListPostVotes(r.Context(), id)
	if err != nil {
		return err
	}
	audit := api.VoteAudit{Voters: make([]api.AuditVoter, 0, len(rows))}
	for _, v := range rows {
		flags := []string{}
		if v.SameIp >= sharedIPThreshold {
			flags = append(flags, "shared_ip")
		}
		if time.Duration(v.CreatedAt-v.GuestCreatedAt)*time.Second < newGuestWindow {
			flags = append(flags, "new_guest")
		}
		hash := v.IpHash
		if len(hash) > 8 {
			hash = hash[:8]
		}
		audit.Voters = append(audit.Voters, api.AuditVoter{
			Guest:  api.AuditGuest{ID: v.GuestID, Name: nullable(v.Name), CreatedAt: ts(v.GuestCreatedAt)},
			IPHash: hash, VotedAt: ts(v.CreatedAt), Flags: flags,
		})
	}
	httpx.WriteJSON(w, http.StatusOK, audit)
	return nil
}

func nullable(s sql.NullString) *string {
	if !s.Valid {
		return nil
	}
	v := s.String
	return &v
}

func (s *Service) handleRemoveVote(w http.ResponseWriter, r *http.Request) error {
	id, err := postID(r)
	if err != nil {
		return err
	}
	guest := r.PathValue("guestId")
	ctx := r.Context()
	if _, err := s.load(ctx, id); err != nil {
		return err
	}
	var count int64
	err = store.InTx(ctx, s.d.DB, func(tx *sql.Tx, q *gen.Queries) error {
		if err := q.DeleteVote(ctx, gen.DeleteVoteParams{PostID: id, GuestID: guest}); err != nil {
			return err
		}
		count, err = q.RecountVotes(ctx, id)
		return err
	})
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, api.VoteState{VoteCount: count, Voted: false})
	return nil
}
