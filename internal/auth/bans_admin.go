package auth

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"patchboard/internal/api"
	"patchboard/internal/httpx"
	"patchboard/internal/store/gen"
)

const maxReason = 200

// RegisterBans adds the admin ban routes.
func (s *Service) RegisterBans(mux *http.ServeMux) {
	mux.Handle("POST /api/admin/bans", s.RequireAdmin(httpx.HandlerFunc(s.handleBan)))
	mux.Handle("GET /api/admin/bans", s.RequireAdmin(httpx.HandlerFunc(s.handleListBans)))
	mux.Handle("DELETE /api/admin/bans/{id}", s.RequireAdmin(httpx.HandlerFunc(s.handleUnban)))
}

func shortHash(h sql.NullString) *string {
	if !h.Valid {
		return nil
	}
	v := h.String
	if len(v) > 8 {
		v = v[:8]
	}
	return &v
}

func (s *Service) handleBan(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		GuestID   string `json:"guestId"`
		IncludeIP bool   `json:"includeIp"`
		Reason    string `json:"reason"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	ctx := r.Context()
	req.Reason = strings.TrimSpace(req.Reason)
	if utf8.RuneCountInString(req.Reason) > maxReason {
		return httpx.Validation().Field("reason", "too_long")
	}
	g, err := s.q.GetGuest(ctx, req.GuestID)
	if errors.Is(err, sql.ErrNoRows) {
		return httpx.Validation().Field("guestId", "invalid")
	} else if err != nil {
		return err
	}
	ip := sql.NullString{}
	if req.IncludeIP {
		ip = sql.NullString{String: g.LastIpHash, Valid: true}
	}
	now := s.clock.Now()
	id, err := s.q.InsertBan(ctx, gen.InsertBanParams{
		GuestID: sql.NullString{String: g.ID, Valid: true}, IpHash: ip, Reason: req.Reason,
		CreatedBy: ActorFrom(ctx).Admin.ID, CreatedAt: now.Unix(),
	})
	if err != nil {
		return err
	}
	gid := g.ID
	httpx.WriteJSON(w, http.StatusCreated, api.Ban{
		ID: id, GuestID: &gid, GuestName: guestFromRow(g).Name, IPHash: shortHash(ip), Reason: req.Reason, CreatedAt: now,
	})
	return nil
}

func (s *Service) handleListBans(w http.ResponseWriter, r *http.Request) error {
	rows, err := s.q.ListBans(r.Context())
	if err != nil {
		return err
	}
	out := make([]api.Ban, 0, len(rows))
	for _, b := range rows {
		ban := api.Ban{ID: b.ID, IPHash: shortHash(b.IpHash), Reason: b.Reason, CreatedAt: time.Unix(b.CreatedAt, 0).UTC()}
		if b.GuestID.Valid {
			gid := b.GuestID.String
			ban.GuestID = &gid
		}
		if b.GuestName.Valid {
			n := b.GuestName.String
			ban.GuestName = &n
		}
		out = append(out, ban)
	}
	httpx.WriteJSON(w, http.StatusOK, out)
	return nil
}

func (s *Service) handleUnban(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		return httpx.ErrNotFound
	}
	if _, err := s.q.GetBan(r.Context(), id); errors.Is(err, sql.ErrNoRows) {
		return httpx.ErrNotFound
	} else if err != nil {
		return err
	}
	if err := s.q.DeleteBan(r.Context(), id); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}
