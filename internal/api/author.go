package api

import (
	"database/sql"
	"strconv"
)

// NewAuthor builds an Author from joined guest/admin columns (exactly one id is valid).
func NewAuthor(guestID, guestName sql.NullString, adminID sql.NullInt64, adminName sql.NullString) Author {
	if adminID.Valid {
		a := Author{Kind: "admin", ID: "admin-" + strconv.FormatInt(adminID.Int64, 10)}
		if adminName.Valid {
			n := adminName.String
			a.Name = &n
		}
		return a
	}
	a := Author{Kind: "guest", ID: guestID.String}
	if guestName.Valid {
		n := guestName.String
		a.Name = &n
	}
	return a
}
