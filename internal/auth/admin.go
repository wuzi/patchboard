package auth

import (
	"context"
	"database/sql"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"patchboard/internal/httpx"
	"patchboard/internal/store/gen"
)

var usernameRe = regexp.MustCompile(`^[a-z0-9_.-]{3,32}$`)

const minPasswordRunes = 10

// validateAdminFields adds field errors for a new or changed admin.
func validateAdminFields(e *httpx.Error, username *string, displayName *string, password *string, passwordField string) {
	if username != nil && !usernameRe.MatchString(*username) {
		e.Field("username", "invalid")
	}
	if displayName != nil {
		switch n := utf8.RuneCountInString(*displayName); {
		case n == 0:
			e.Field("displayName", "required")
		case n > maxNameRunes:
			e.Field("displayName", "too_long")
		}
	}
	if password != nil && utf8.RuneCountInString(*password) < minPasswordRunes {
		e.Field(passwordField, "too_short")
	}
}

// CreateAdmin validates and inserts an admin account, returning its id.
func CreateAdmin(ctx context.Context, db *sql.DB, now time.Time, username, displayName, password string) (int64, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	id, err := createAdminTx(ctx, gen.New(tx), now, username, displayName, password)
	if err != nil {
		return 0, err
	}
	return id, tx.Commit()
}

func createAdminTx(ctx context.Context, q *gen.Queries, now time.Time, username, displayName, password string) (int64, error) {
	username = strings.ToLower(strings.TrimSpace(username))
	displayName = strings.TrimSpace(displayName)
	e := httpx.Validation()
	validateAdminFields(e, &username, &displayName, &password, "password")
	if len(e.Fields) > 0 {
		return 0, e
	}
	if _, err := q.GetAdminByUsername(ctx, username); err == nil {
		return 0, httpx.E(http.StatusConflict, "username_taken", "username is already taken").Field("username", "taken")
	} else if !isNoRows(err) {
		return 0, err
	}
	hash, err := HashPassword(password)
	if err != nil {
		return 0, err
	}
	return q.InsertAdmin(ctx, gen.InsertAdminParams{Username: username, DisplayName: displayName, PasswordHash: hash, CreatedAt: now.Unix()})
}

// ResetPassword sets a new password and signs the admin out everywhere.
func ResetPassword(ctx context.Context, db *sql.DB, username, password string) error {
	e := httpx.Validation()
	validateAdminFields(e, nil, nil, &password, "password")
	if len(e.Fields) > 0 {
		return e
	}
	q := gen.New(db)
	a, err := q.GetAdminByUsername(ctx, strings.ToLower(strings.TrimSpace(username)))
	if isNoRows(err) {
		return httpx.ErrNotFound
	} else if err != nil {
		return err
	}
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	if err := q.SetAdminPassword(ctx, gen.SetAdminPasswordParams{PasswordHash: hash, ID: a.ID}); err != nil {
		return err
	}
	return q.DeleteAdminSessions(ctx, a.ID)
}
