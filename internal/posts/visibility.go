package posts

import (
	"context"
	"database/sql"
	"errors"

	"patchboard/internal/auth"
	"patchboard/internal/store/gen"
)

type postRow = gen.GetPostFullRow

func isAuthor(p postRow, viewer *auth.Actor) bool {
	if viewer.Admin != nil && p.AuthorAdminID.Valid && p.AuthorAdminID.Int64 == viewer.Admin.ID {
		return true
	}
	return viewer.Guest != nil && p.AuthorGuestID.Valid && p.AuthorGuestID.String == viewer.Guest.ID
}

// visible: deleted posts only for admins; pending posts for their author and admins.
func visible(p postRow, viewer *auth.Actor) bool {
	if p.DeletedAt.Valid {
		return viewer.IsAdmin()
	}
	if p.Pending != 0 {
		return viewer.IsAdmin() || isAuthor(p, viewer)
	}
	return true
}

// canEdit: admins always (unless merged); authors while the post sits in the default status.
func canEdit(p postRow, viewer *auth.Actor, defaultStatusID int64) bool {
	if p.MergedIntoID.Valid {
		return false
	}
	if viewer.IsAdmin() {
		return true
	}
	return isAuthor(p, viewer) && !p.DeletedAt.Valid && p.StatusID == defaultStatusID
}

// VisibleTo reports whether viewer may see (and so interact with) the post; merged posts are not.
func (s *Service) VisibleTo(ctx context.Context, postID int64, viewer *auth.Actor) (bool, error) {
	p, err := s.q.GetPostFull(ctx, postID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return !p.MergedIntoID.Valid && visible(p, viewer), nil
}
