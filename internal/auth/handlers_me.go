package auth

import (
	"database/sql"
	"net/http"
	"strings"
	"unicode/utf8"

	"patchboard/internal/api"
	"patchboard/internal/httpx"
	"patchboard/internal/store/gen"
)

const maxNameRunes = 40

// Register adds the guest identity routes.
func (s *Service) Register(mux *http.ServeMux) {
	mux.Handle("GET /api/me", httpx.HandlerFunc(s.handleMe))
	mux.Handle("PUT /api/me/name", httpx.HandlerFunc(s.handleSetName))
	mux.Handle("GET /r/{token}", http.HandlerFunc(s.handleRecover))
}

// MeFor builds the /api/me payload for an actor.
func (s *Service) MeFor(a *Actor) api.Me {
	var m api.Me
	if a.Guest != nil {
		m.Guest = &api.MeGuest{ID: a.Guest.ID, Name: a.Guest.Name, RecoveryURL: s.cfg.BaseURL.String() + "/r/" + a.guestToken}
	}
	if a.Admin != nil {
		m.Admin = &api.MeAdmin{ID: a.Admin.ID, Username: a.Admin.Username, DisplayName: a.Admin.DisplayName}
	}
	return m
}

func (s *Service) handleMe(w http.ResponseWriter, r *http.Request) error {
	httpx.WriteJSON(w, http.StatusOK, s.MeFor(ActorFrom(r.Context())))
	return nil
}

func (s *Service) handleSetName(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		Name string `json:"name"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	name := strings.TrimSpace(req.Name)
	if utf8.RuneCountInString(name) > maxNameRunes {
		return httpx.Validation().Field("name", "too_long")
	}
	g, err := s.GuestWriter(w, r)
	if err != nil {
		return err
	}
	if err := s.q.SetGuestName(r.Context(), gen.SetGuestNameParams{
		Name: sql.NullString{String: name, Valid: name != ""}, ID: g.ID,
	}); err != nil {
		return err
	}
	if name == "" {
		g.Name = nil
	} else {
		g.Name = &name
	}
	httpx.WriteJSON(w, http.StatusOK, s.MeFor(ActorFrom(r.Context())))
	return nil
}

func (s *Service) handleRecover(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if _, err := s.q.GetGuestByTokenHash(r.Context(), hashToken(token)); err != nil {
		http.Redirect(w, r, "/?recovered=0", http.StatusFound)
		return
	}
	s.setCookie(w, guestCookie, token, guestMaxAge)
	http.Redirect(w, r, "/?recovered=1", http.StatusFound)
}
