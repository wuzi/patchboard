package posts

import (
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
