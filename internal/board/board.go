// Package board owns the board settings, statuses and tags.
package board

import (
	"context"
	"database/sql"
	"errors"

	"patchboard/internal/api"
	"patchboard/internal/store/gen"
)

type Service struct {
	db *sql.DB
	q  *gen.Queries
}

func NewService(db *sql.DB) *Service { return &Service{db: db, q: gen.New(db)} }

// Get returns the public board payload.
func (s *Service) Get(ctx context.Context) (api.Board, error) {
	st, err := s.q.GetSettings(ctx)
	if err != nil {
		return api.Board{}, err
	}
	b := api.Board{
		Title:           api.I18nName{PT: st.TitlePt, EN: st.TitleEn},
		Description:     api.I18nName{PT: st.DescriptionPt, EN: st.DescriptionEn},
		TeamName:        st.TeamName,
		AccentColor:     st.AccentColor,
		RequireApproval: st.RequireApproval != 0,
		UploadsEnabled:  st.UploadsEnabled != 0,
	}
	if st.LogoUploadID.Valid {
		ext, err := s.q.GetLogoExt(ctx)
		if err == nil {
			u := "/uploads/" + st.LogoUploadID.String + "." + ext
			b.LogoURL = &u
		} else if !errors.Is(err, sql.ErrNoRows) {
			return b, err
		}
	}
	if b.Statuses, err = s.Statuses(ctx); err != nil {
		return b, err
	}
	b.Tags, err = s.Tags(ctx)
	return b, err
}

// Statuses lists statuses by position.
func (s *Service) Statuses(ctx context.Context) ([]api.Status, error) {
	rows, err := s.q.ListStatuses(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]api.Status, 0, len(rows))
	for _, r := range rows {
		out = append(out, statusJSON(r))
	}
	return out, nil
}

// Tags lists tags by position.
func (s *Service) Tags(ctx context.Context) ([]api.Tag, error) {
	rows, err := s.q.ListTags(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]api.Tag, 0, len(rows))
	for _, r := range rows {
		out = append(out, tagJSON(r))
	}
	return out, nil
}

func statusJSON(r gen.Status) api.Status {
	return api.Status{
		ID: r.ID, Name: api.I18nName{PT: r.NamePt, EN: r.NameEn}, Color: r.Color, Position: r.Position,
		ShowOnRoadmap: r.ShowOnRoadmap != 0, IsDefault: r.IsDefault != 0,
	}
}

func tagJSON(r gen.Tag) api.Tag {
	return api.Tag{ID: r.ID, Name: api.I18nName{PT: r.NamePt, EN: r.NameEn}, Position: r.Position}
}

// DefaultStatusID returns the status new posts land in.
func (s *Service) DefaultStatusID(ctx context.Context) (int64, error) {
	rows, err := s.q.ListStatuses(ctx)
	if err != nil {
		return 0, err
	}
	for _, r := range rows {
		if r.IsDefault != 0 {
			return r.ID, nil
		}
	}
	return 0, errors.New("no default status")
}

// StatusExists reports whether a status id exists.
func (s *Service) StatusExists(ctx context.Context, id int64) (bool, error) {
	return exists(s.q.GetStatus(ctx, id))
}

// TagExists reports whether a tag id exists.
func (s *Service) TagExists(ctx context.Context, id int64) (bool, error) {
	return exists(s.q.GetTag(ctx, id))
}

// Settings returns the raw settings row.
func (s *Service) Settings(ctx context.Context) (gen.BoardSetting, error) {
	return s.q.GetSettings(ctx)
}

func exists[T any](_ T, err error) (bool, error) {
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}
