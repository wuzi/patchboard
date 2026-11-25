package auth

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"
	"time"

	"patchboard/internal/api"
	"patchboard/internal/httpx"
	"patchboard/internal/store/gen"
)

// RegisterAdmin adds the admin account routes.
func (s *Service) RegisterAdmin(mux *http.ServeMux) {
	mux.Handle("POST /api/admin/login", httpx.HandlerFunc(s.handleLogin))
	mux.Handle("GET /api/admin/invites/{token}", httpx.HandlerFunc(s.handleCheckInvite))
	mux.Handle("POST /api/admin/invites/{token}/accept", httpx.HandlerFunc(s.handleAcceptInvite))
	mux.Handle("POST /api/admin/logout", s.RequireAdmin(httpx.HandlerFunc(s.handleLogout)))
	mux.Handle("POST /api/admin/invites", s.RequireAdmin(httpx.HandlerFunc(s.handleCreateInvite)))
	mux.Handle("GET /api/admin/team", s.RequireAdmin(httpx.HandlerFunc(s.handleTeam)))
	mux.Handle("POST /api/admin/team/{id}/disable", s.RequireAdmin(httpx.HandlerFunc(s.handleDisable)))
	mux.Handle("PATCH /api/admin/me", s.RequireAdmin(httpx.HandlerFunc(s.handleUpdateMe)))
}

func invalidCredentials() *httpx.Error {
	return httpx.E(http.StatusUnauthorized, "invalid_credentials", "wrong username or password")
}

func (s *Service) handleLogin(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	a := ActorFrom(r.Context())
	username := strings.ToLower(strings.TrimSpace(req.Username))
	if ok, retry := s.lim.Allow(httpx.RuleLoginIP, a.Info.IPHash); !ok {
		return httpx.RateLimited(retry)
	}
	if ok, retry := s.lim.Allow(httpx.RuleLoginUser, username); !ok {
		return httpx.RateLimited(retry)
	}
	row, err := s.q.GetAdminByUsername(r.Context(), username)
	if isNoRows(err) {
		VerifyPassword(dummyHash, req.Password)
		return invalidCredentials()
	} else if err != nil {
		return err
	}
	ok, err := VerifyPassword(row.PasswordHash, req.Password)
	if err != nil {
		return err
	}
	if !ok || row.DisabledAt.Valid {
		return invalidCredentials()
	}
	if err := s.startSession(r.Context(), w, s.q, row.ID); err != nil {
		return err
	}
	a.Admin = &Admin{ID: row.ID, Username: row.Username, DisplayName: row.DisplayName}
	httpx.WriteJSON(w, http.StatusOK, s.MeFor(a))
	return nil
}

func (s *Service) handleLogout(w http.ResponseWriter, r *http.Request) error {
	if ck, err := r.Cookie(adminCookie); err == nil {
		if err := s.q.DeleteSession(r.Context(), hashToken(ck.Value)); err != nil {
			return err
		}
	}
	s.clearCookie(w, adminCookie)
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Service) handleCreateInvite(w http.ResponseWriter, r *http.Request) error {
	token := randomToken()
	now := s.clock.Now()
	exp := now.Add(inviteLength)
	if err := s.q.CreateInvite(r.Context(), gen.CreateInviteParams{
		TokenHash: hashToken(token), CreatedBy: ActorFrom(r.Context()).Admin.ID, CreatedAt: now.Unix(), ExpiresAt: exp.Unix(),
	}); err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusCreated, api.Invite{URL: s.cfg.BaseURL.String() + "/admin/invite/" + token, ExpiresAt: exp})
	return nil
}

func (s *Service) handleCheckInvite(w http.ResponseWriter, r *http.Request) error {
	_, err := s.inviteState(r.Context(), s.q, r.PathValue("token"))
	if err != nil {
		if _, ok := err.(*httpx.Error); !ok {
			return err
		}
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]bool{"valid": err == nil})
	return nil
}

func (s *Service) handleAcceptInvite(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		Username    string `json:"username"`
		DisplayName string `json:"displayName"`
		Password    string `json:"password"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	token := r.PathValue("token")
	ctx := r.Context()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	q := gen.New(tx)
	if _, err := s.inviteState(ctx, q, token); err != nil {
		return err
	}
	now := s.clock.Now()
	id, err := createAdminTx(ctx, q, now, req.Username, req.DisplayName, req.Password)
	if err != nil {
		return err
	}
	if err := q.UseInvite(ctx, gen.UseInviteParams{
		UsedAt: sql.NullInt64{Int64: now.Unix(), Valid: true}, UsedBy: sql.NullInt64{Int64: id, Valid: true}, TokenHash: hashToken(token),
	}); err != nil {
		return err
	}
	if err := s.startSession(ctx, w, q, id); err != nil {
		return err
	}
	row, err := q.GetAdmin(ctx, id)
	if err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	a := ActorFrom(ctx)
	a.Admin = &Admin{ID: row.ID, Username: row.Username, DisplayName: row.DisplayName}
	httpx.WriteJSON(w, http.StatusCreated, s.MeFor(a))
	return nil
}

func (s *Service) handleTeam(w http.ResponseWriter, r *http.Request) error {
	rows, err := s.q.ListAdmins(r.Context())
	if err != nil {
		return err
	}
	out := make([]api.TeamMember, 0, len(rows))
	for _, a := range rows {
		out = append(out, api.TeamMember{
			ID: a.ID, Username: a.Username, DisplayName: a.DisplayName,
			CreatedAt: time.Unix(a.CreatedAt, 0).UTC(), Disabled: a.DisabledAt.Valid,
		})
	}
	httpx.WriteJSON(w, http.StatusOK, out)
	return nil
}

func (s *Service) handleDisable(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		return httpx.ErrNotFound
	}
	if id == ActorFrom(r.Context()).Admin.ID {
		return httpx.E(http.StatusUnprocessableEntity, "cannot_disable_self", "you cannot disable your own account")
	}
	if _, err := s.q.GetAdmin(r.Context(), id); isNoRows(err) {
		return httpx.ErrNotFound
	} else if err != nil {
		return err
	}
	return storeTx(r, s, func(q *gen.Queries) error {
		if err := q.DisableAdmin(r.Context(), gen.DisableAdminParams{DisabledAt: sql.NullInt64{Int64: s.clock.Now().Unix(), Valid: true}, ID: id}); err != nil {
			return err
		}
		if err := q.DeleteAdminSessions(r.Context(), id); err != nil {
			return err
		}
		w.WriteHeader(http.StatusNoContent)
		return nil
	})
}

func (s *Service) handleUpdateMe(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		DisplayName     *string `json:"displayName"`
		CurrentPassword *string `json:"currentPassword"`
		NewPassword     *string `json:"newPassword"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	a := ActorFrom(r.Context())
	ctx := r.Context()
	e := httpx.Validation()
	if req.DisplayName != nil {
		n := strings.TrimSpace(*req.DisplayName)
		req.DisplayName = &n
	}
	validateAdminFields(e, nil, req.DisplayName, req.NewPassword, "newPassword")
	if req.NewPassword != nil {
		row, err := s.q.GetAdmin(ctx, a.Admin.ID)
		if err != nil {
			return err
		}
		ok := false
		if req.CurrentPassword != nil {
			if ok, err = VerifyPassword(row.PasswordHash, *req.CurrentPassword); err != nil {
				return err
			}
		}
		if !ok {
			e.Field("currentPassword", "invalid")
		}
	}
	if len(e.Fields) > 0 {
		return e
	}
	if req.DisplayName != nil {
		if err := s.q.SetAdminDisplayName(ctx, gen.SetAdminDisplayNameParams{DisplayName: *req.DisplayName, ID: a.Admin.ID}); err != nil {
			return err
		}
		a.Admin.DisplayName = *req.DisplayName
	}
	if req.NewPassword != nil {
		hash, err := HashPassword(*req.NewPassword)
		if err != nil {
			return err
		}
		if err := s.q.SetAdminPassword(ctx, gen.SetAdminPasswordParams{PasswordHash: hash, ID: a.Admin.ID}); err != nil {
			return err
		}
	}
	httpx.WriteJSON(w, http.StatusOK, s.MeFor(a))
	return nil
}

func storeTx(r *http.Request, s *Service, fn func(q *gen.Queries) error) error {
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := fn(gen.New(tx)); err != nil {
		return err
	}
	return tx.Commit()
}
