package auth

import (
	"context"
	"net/http"
	"time"

	"patchboard/internal/httpx"
	"patchboard/internal/store/gen"
)

const inviteLength = 7 * 24 * time.Hour

// inviteState returns nil when the invite can be used, else the 404/410 error.
func (s *Service) inviteState(ctx context.Context, q *gen.Queries, token string) (gen.AdminInvite, error) {
	inv, err := q.GetInvite(ctx, hashToken(token))
	if isNoRows(err) {
		return inv, httpx.ErrNotFound
	} else if err != nil {
		return inv, err
	}
	if inv.UsedAt.Valid {
		return inv, httpx.E(http.StatusGone, "invite_used", "this invite was already used")
	}
	if inv.ExpiresAt <= s.clock.Now().Unix() {
		return inv, httpx.E(http.StatusGone, "invite_expired", "this invite has expired")
	}
	return inv, nil
}
