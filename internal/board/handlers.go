package board

import (
	"database/sql"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"patchboard/internal/api"
	"patchboard/internal/httpx"
	"patchboard/internal/store"
	"patchboard/internal/store/gen"
)

var colorRe = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

// Register adds the public board route and the admin settings/status/tag routes.
func (s *Service) Register(mux *http.ServeMux, requireAdmin func(http.Handler) http.Handler) {
	admin := func(f httpx.HandlerFunc) http.Handler { return requireAdmin(f) }
	mux.Handle("GET /api/board", httpx.HandlerFunc(s.handleBoard))
	mux.Handle("PUT /api/admin/settings", admin(s.handleSettings))
	mux.Handle("POST /api/admin/statuses", admin(s.handleCreateStatus))
	mux.Handle("PUT /api/admin/statuses/order", admin(s.handleOrderStatuses))
	mux.Handle("PATCH /api/admin/statuses/{id}", admin(s.handleUpdateStatus))
	mux.Handle("DELETE /api/admin/statuses/{id}", admin(s.handleDeleteStatus))
	mux.Handle("POST /api/admin/tags", admin(s.handleCreateTag))
	mux.Handle("PUT /api/admin/tags/order", admin(s.handleOrderTags))
	mux.Handle("PATCH /api/admin/tags/{id}", admin(s.handleUpdateTag))
	mux.Handle("DELETE /api/admin/tags/{id}", admin(s.handleDeleteTag))
}

func (s *Service) handleBoard(w http.ResponseWriter, r *http.Request) error {
	b, err := s.Get(r.Context())
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, b)
	return nil
}

// checkText records required/too_long for a trimmed string.
func checkText(e *httpx.Error, field, v string, required bool, max int) {
	n := utf8.RuneCountInString(v)
	if required && n == 0 {
		e.Field(field, "required")
	} else if n > max {
		e.Field(field, "too_long")
	}
}

func trimName(n api.I18nName) api.I18nName {
	return api.I18nName{PT: strings.TrimSpace(n.PT), EN: strings.TrimSpace(n.EN)}
}

func (s *Service) handleSettings(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		Title           api.I18nName `json:"title"`
		Description     api.I18nName `json:"description"`
		TeamName        string       `json:"teamName"`
		AccentColor     string       `json:"accentColor"`
		LogoUploadID    *string      `json:"logoUploadId"`
		RequireApproval bool         `json:"requireApproval"`
		UploadsEnabled  bool         `json:"uploadsEnabled"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	req.Title, req.Description = trimName(req.Title), trimName(req.Description)
	req.TeamName = strings.TrimSpace(req.TeamName)
	e := httpx.Validation()
	checkText(e, "title.pt", req.Title.PT, true, 80)
	checkText(e, "title.en", req.Title.EN, true, 80)
	checkText(e, "description.pt", req.Description.PT, false, 500)
	checkText(e, "description.en", req.Description.EN, false, 500)
	checkText(e, "teamName", req.TeamName, true, 40)
	if !colorRe.MatchString(req.AccentColor) {
		e.Field("accentColor", "invalid")
	}
	logo := sql.NullString{}
	if req.LogoUploadID != nil {
		if ok, err := exists(s.q.GetUpload(r.Context(), *req.LogoUploadID)); err != nil {
			return err
		} else if !ok {
			e.Field("logoUploadId", "invalid")
		}
		logo = sql.NullString{String: *req.LogoUploadID, Valid: true}
	}
	if len(e.Fields) > 0 {
		return e
	}
	if err := s.q.UpdateSettings(r.Context(), gen.UpdateSettingsParams{
		TitlePt: req.Title.PT, TitleEn: req.Title.EN, DescriptionPt: req.Description.PT, DescriptionEn: req.Description.EN,
		TeamName: req.TeamName, AccentColor: req.AccentColor, LogoUploadID: logo,
		RequireApproval: boolInt(req.RequireApproval), UploadsEnabled: boolInt(req.UploadsEnabled),
	}); err != nil {
		return err
	}
	return s.handleBoard(w, r)
}

func boolInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

func checkName(e *httpx.Error, n api.I18nName) {
	checkText(e, "name.pt", n.PT, true, 30)
	checkText(e, "name.en", n.EN, true, 30)
}

func pathID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		return 0, httpx.ErrNotFound
	}
	return id, nil
}

func (s *Service) handleCreateStatus(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		Name          api.I18nName `json:"name"`
		Color         string       `json:"color"`
		ShowOnRoadmap bool         `json:"showOnRoadmap"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	req.Name = trimName(req.Name)
	e := httpx.Validation()
	checkName(e, req.Name)
	if !colorRe.MatchString(req.Color) {
		e.Field("color", "invalid")
	}
	if len(e.Fields) > 0 {
		return e
	}
	all, err := s.q.ListStatuses(r.Context())
	if err != nil {
		return err
	}
	row, err := s.q.InsertStatus(r.Context(), gen.InsertStatusParams{
		NamePt: req.Name.PT, NameEn: req.Name.EN, Color: req.Color, Position: int64(len(all)), ShowOnRoadmap: boolInt(req.ShowOnRoadmap),
	})
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusCreated, statusJSON(row))
	return nil
}

func (s *Service) handleUpdateStatus(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	var req struct {
		Name          *api.I18nName `json:"name"`
		Color         *string       `json:"color"`
		ShowOnRoadmap *bool         `json:"showOnRoadmap"`
		IsDefault     *bool         `json:"isDefault"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	return store.InTx(r.Context(), s.db, func(tx *sql.Tx, q *gen.Queries) error {
		cur, err := q.GetStatus(r.Context(), id)
		if errors.Is(err, sql.ErrNoRows) {
			return httpx.ErrNotFound
		} else if err != nil {
			return err
		}
		e := httpx.Validation()
		if req.Name != nil {
			n := trimName(*req.Name)
			checkName(e, n)
			cur.NamePt, cur.NameEn = n.PT, n.EN
		}
		if req.Color != nil {
			if !colorRe.MatchString(*req.Color) {
				e.Field("color", "invalid")
			}
			cur.Color = *req.Color
		}
		if req.ShowOnRoadmap != nil {
			cur.ShowOnRoadmap = boolInt(*req.ShowOnRoadmap)
		}
		if req.IsDefault != nil && !*req.IsDefault && cur.IsDefault != 0 {
			e.Field("isDefault", "invalid") // pick another default instead
		}
		if len(e.Fields) > 0 {
			return e
		}
		if err := q.UpdateStatus(r.Context(), gen.UpdateStatusParams{
			NamePt: cur.NamePt, NameEn: cur.NameEn, Color: cur.Color, ShowOnRoadmap: cur.ShowOnRoadmap, ID: id,
		}); err != nil {
			return err
		}
		if req.IsDefault != nil && *req.IsDefault {
			if err := q.ClearDefaultStatus(r.Context()); err != nil {
				return err
			}
			if err := q.SetDefaultStatus(r.Context(), id); err != nil {
				return err
			}
		}
		row, err := q.GetStatus(r.Context(), id)
		if err != nil {
			return err
		}
		httpx.WriteJSON(w, http.StatusOK, statusJSON(row))
		return nil
	})
}

// samePermutation reports whether ids is exactly a reordering of want.
func samePermutation(ids []int64, want []int64) bool {
	if len(ids) != len(want) {
		return false
	}
	seen := map[int64]bool{}
	for _, id := range want {
		seen[id] = true
	}
	for _, id := range ids {
		if !seen[id] {
			return false
		}
		delete(seen, id)
	}
	return len(seen) == 0
}

func (s *Service) handleOrderStatuses(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		IDs []int64 `json:"ids"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	err := store.InTx(r.Context(), s.db, func(tx *sql.Tx, q *gen.Queries) error {
		all, err := q.ListStatuses(r.Context())
		if err != nil {
			return err
		}
		have := make([]int64, len(all))
		for i, st := range all {
			have[i] = st.ID
		}
		if !samePermutation(req.IDs, have) {
			return httpx.E(http.StatusUnprocessableEntity, "invalid_order", "ids must list every status exactly once")
		}
		for i, id := range req.IDs {
			if err := q.SetStatusPosition(r.Context(), gen.SetStatusPositionParams{Position: int64(i), ID: id}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	out, err := s.Statuses(r.Context())
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, out)
	return nil
}

func (s *Service) handleDeleteStatus(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	return store.InTx(r.Context(), s.db, func(tx *sql.Tx, q *gen.Queries) error {
		cur, err := q.GetStatus(r.Context(), id)
		if errors.Is(err, sql.ErrNoRows) {
			return httpx.ErrNotFound
		} else if err != nil {
			return err
		}
		if cur.IsDefault != 0 {
			return httpx.E(http.StatusUnprocessableEntity, "cannot_delete_default", "choose another default status first")
		}
		used, err := q.CountStatusUsage(r.Context(), id)
		if err != nil {
			return err
		}
		if used > 0 {
			raw := r.URL.Query().Get("moveTo")
			if raw == "" {
				return httpx.E(http.StatusUnprocessableEntity, "move_required", "choose a status to move this status's posts to")
			}
			to, err := strconv.ParseInt(raw, 10, 64)
			if err != nil || to == id {
				return httpx.Validation().Field("moveTo", "invalid")
			}
			if ok, err := exists(q.GetStatus(r.Context(), to)); err != nil {
				return err
			} else if !ok {
				return httpx.Validation().Field("moveTo", "invalid")
			}
			if err := q.MovePostsStatus(r.Context(), gen.MovePostsStatusParams{ToID: to, FromID: id}); err != nil {
				return err
			}
			if err := q.MoveUpdateLinksStatus(r.Context(), gen.MoveUpdateLinksStatusParams{ToID: to, FromID: id}); err != nil {
				return err
			}
		}
		if err := q.DeleteStatus(r.Context(), id); err != nil {
			return err
		}
		rest, err := q.ListStatuses(r.Context())
		if err != nil {
			return err
		}
		for i, st := range rest {
			if err := q.SetStatusPosition(r.Context(), gen.SetStatusPositionParams{Position: int64(i), ID: st.ID}); err != nil {
				return err
			}
		}
		w.WriteHeader(http.StatusNoContent)
		return nil
	})
}

func (s *Service) handleCreateTag(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		Name api.I18nName `json:"name"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	req.Name = trimName(req.Name)
	e := httpx.Validation()
	checkName(e, req.Name)
	if len(e.Fields) > 0 {
		return e
	}
	all, err := s.q.ListTags(r.Context())
	if err != nil {
		return err
	}
	row, err := s.q.InsertTag(r.Context(), gen.InsertTagParams{NamePt: req.Name.PT, NameEn: req.Name.EN, Position: int64(len(all))})
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusCreated, tagJSON(row))
	return nil
}

func (s *Service) handleUpdateTag(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	var req struct {
		Name api.I18nName `json:"name"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	req.Name = trimName(req.Name)
	e := httpx.Validation()
	checkName(e, req.Name)
	if len(e.Fields) > 0 {
		return e
	}
	if ok, err := s.TagExists(r.Context(), id); err != nil {
		return err
	} else if !ok {
		return httpx.ErrNotFound
	}
	if err := s.q.UpdateTag(r.Context(), gen.UpdateTagParams{NamePt: req.Name.PT, NameEn: req.Name.EN, ID: id}); err != nil {
		return err
	}
	row, err := s.q.GetTag(r.Context(), id)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, tagJSON(row))
	return nil
}

func (s *Service) handleOrderTags(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		IDs []int64 `json:"ids"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	err := store.InTx(r.Context(), s.db, func(tx *sql.Tx, q *gen.Queries) error {
		all, err := q.ListTags(r.Context())
		if err != nil {
			return err
		}
		have := make([]int64, len(all))
		for i, t := range all {
			have[i] = t.ID
		}
		if !samePermutation(req.IDs, have) {
			return httpx.E(http.StatusUnprocessableEntity, "invalid_order", "ids must list every tag exactly once")
		}
		for i, id := range req.IDs {
			if err := q.SetTagPosition(r.Context(), gen.SetTagPositionParams{Position: int64(i), ID: id}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	out, err := s.Tags(r.Context())
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, out)
	return nil
}

func (s *Service) handleDeleteTag(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	if ok, err := s.TagExists(r.Context(), id); err != nil {
		return err
	} else if !ok {
		return httpx.ErrNotFound
	}
	if err := s.q.DeleteTag(r.Context(), id); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}
