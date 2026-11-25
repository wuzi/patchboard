package server

import (
	"context"
	"database/sql"

	"patchboard/internal/api"
	"patchboard/internal/auth"
	"patchboard/internal/httpx"
)

// noComments / noImages stand in until comments and uploads are wired.
func noComments(context.Context, int64, *auth.Actor) ([]api.Comment, error) {
	return []api.Comment{}, nil
}

type noImages struct{}

func (noImages) ForPost(context.Context, int64) ([]api.Image, error) { return []api.Image{}, nil }
func (noImages) Attach(_ context.Context, _ *sql.Tx, _ int64, ids []string, _ auth.AuthorRef) error {
	if len(ids) > 0 {
		return httpx.Validation().Field("imageIds", "invalid")
	}
	return nil
}
