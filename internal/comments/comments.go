// Package comments owns comments, replies and likes.
package comments

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"patchboard/internal/api"
	"patchboard/internal/auth"
	"patchboard/internal/clock"
	"patchboard/internal/httpx"
	"patchboard/internal/markdown"
	"patchboard/internal/posts"
	"patchboard/internal/store"
	"patchboard/internal/store/gen"
)

const maxComment = 5000

type Service struct {
	db    *sql.DB
	q     *gen.Queries
	clock clock.Clock
	auth  *auth.Service
	posts *posts.Service
}

func NewService(db *sql.DB, c clock.Clock, a *auth.Service, p *posts.Service) *Service {
	return &Service{db: db, q: gen.New(db), clock: c, auth: a, posts: p}
}

// row is the shape shared by ListPostComments and GetComment.
type row struct {
	ID            int64
	PostID        int64
	ParentID      sql.NullInt64
	AuthorGuestID sql.NullString
	AuthorAdminID sql.NullInt64
	BodyMd        string
	BodyHtml      string
	LikeCount     int64
	CreatedAt     int64
	DeletedAt     sql.NullInt64
	GuestName     sql.NullString
	AdminName     string
}

func isAuthor(c row, viewer *auth.Actor) bool {
	if viewer.Admin != nil && c.AuthorAdminID.Valid && c.AuthorAdminID.Int64 == viewer.Admin.ID {
		return true
	}
	return viewer.Guest != nil && c.AuthorGuestID.Valid && c.AuthorGuestID.String == viewer.Guest.ID
}

func canEdit(c row, viewer *auth.Actor) bool {
	return !c.DeletedAt.Valid && (viewer.IsAdmin() || isAuthor(c, viewer))
}

func toJSON(c row, viewer *auth.Actor, liked bool) api.Comment {
	out := api.Comment{ID: c.ID, LikeCount: c.LikeCount, Liked: liked, CreatedAt: time.Unix(c.CreatedAt, 0).UTC(), Replies: []api.Comment{}}
	if c.ParentID.Valid {
		p := c.ParentID.Int64
		out.ParentID = &p
	}
	if c.DeletedAt.Valid {
		out.Deleted = true
		return out
	}
	a := api.NewAuthor(c.AuthorGuestID, c.GuestName, c.AuthorAdminID, sql.NullString{String: c.AdminName, Valid: c.AuthorAdminID.Valid})
	out.Author = &a
	html := c.BodyHtml
	out.BodyHTML = &html
	out.CanEdit = canEdit(c, viewer)
	if out.CanEdit {
		md := c.BodyMd
		out.BodyMD = &md
	}
	return out
}

// ForPost returns the comment tree: top-level oldest first, replies oldest first.
func (s *Service) ForPost(ctx context.Context, postID int64, viewer *auth.Actor) ([]api.Comment, error) {
	rows, err := s.q.ListPostComments(ctx, postID)
	if err != nil {
		return nil, err
	}
	liked := map[int64]bool{}
	if viewer.Guest != nil {
		ids, err := s.q.LikedByGuest(ctx, gen.LikedByGuestParams{PostID: postID, GuestID: viewer.Guest.ID})
		if err != nil {
			return nil, err
		}
		for _, id := range ids {
			liked[id] = true
		}
	}
	replies := map[int64][]api.Comment{}
	var top []row
	for _, r := range rows {
		c := row(r)
		if c.ParentID.Valid {
			if !c.DeletedAt.Valid {
				replies[c.ParentID.Int64] = append(replies[c.ParentID.Int64], toJSON(c, viewer, liked[c.ID]))
			}
			continue
		}
		top = append(top, c)
	}
	out := []api.Comment{}
	for _, c := range top {
		kids := replies[c.ID]
		if c.DeletedAt.Valid && len(kids) == 0 {
			continue
		}
		j := toJSON(c, viewer, liked[c.ID])
		if kids != nil {
			j.Replies = kids
		}
		out = append(out, j)
	}
	return out, nil
}

func checkBody(body string) *httpx.Error {
	switch n := utf8.RuneCountInString(body); {
	case n == 0:
		return httpx.Validation().Field("body", "too_short")
	case n > maxComment:
		return httpx.Validation().Field("body", "too_long")
	}
	return nil
}

// visiblePost returns 404 unless the viewer can see the post.
func (s *Service) visiblePost(ctx context.Context, postID int64, viewer *auth.Actor) error {
	ok, err := s.posts.VisibleTo(ctx, postID, viewer)
	if err != nil {
		return err
	}
	if !ok {
		return httpx.ErrNotFound
	}
	return nil
}

func (s *Service) handleCreate(w http.ResponseWriter, r *http.Request) error {
	postID, err := pathID(r)
	if err != nil {
		return err
	}
	var req struct {
		Body     string `json:"body"`
		ParentID *int64 `json:"parentId"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	ctx := r.Context()
	viewer := auth.ActorFrom(ctx)
	if err := s.visiblePost(ctx, postID, viewer); err != nil {
		return err
	}
	locked, err := s.q.GetPostLock(ctx, postID)
	if err != nil {
		return err
	}
	if locked != 0 && !viewer.IsAdmin() {
		return httpx.E(http.StatusForbidden, "locked", "comments are closed on this post")
	}
	req.Body = strings.TrimSpace(req.Body)
	if e := checkBody(req.Body); e != nil {
		return e
	}
	var parent sql.NullInt64
	if req.ParentID != nil {
		p, err := s.q.GetComment(ctx, *req.ParentID)
		if errors.Is(err, sql.ErrNoRows) || (err == nil && (p.PostID != postID || p.DeletedAt.Valid)) {
			return httpx.Validation().Field("parentId", "invalid")
		} else if err != nil {
			return err
		}
		parent = sql.NullInt64{Int64: p.ID, Valid: true}
		if p.ParentID.Valid {
			parent = p.ParentID // one level of replies
		}
	}
	if err := s.auth.Limit(r, httpx.RuleComment); err != nil {
		return err
	}
	author, err := s.auth.Writer(w, r)
	if err != nil {
		return err
	}
	now := s.clock.Now().Unix()
	var id int64
	err = store.InTx(ctx, s.db, func(tx *sql.Tx, q *gen.Queries) error {
		guest := sql.NullString{}
		if author.GuestID != nil {
			guest = sql.NullString{String: *author.GuestID, Valid: true}
		}
		admin := sql.NullInt64{}
		if author.AdminID != nil {
			admin = sql.NullInt64{Int64: *author.AdminID, Valid: true}
		}
		id, err = q.InsertComment(ctx, gen.InsertCommentParams{
			PostID: postID, ParentID: parent, AuthorGuestID: guest, AuthorAdminID: admin,
			BodyMd: req.Body, BodyHtml: markdown.Render(req.Body), IpHash: viewer.Info.IPHash, CreatedAt: now, UpdatedAt: now,
		})
		if err != nil {
			return err
		}
		return q.RecountComments(ctx, postID)
	})
	if err != nil {
		return err
	}
	return s.writeComment(w, r, http.StatusCreated, id)
}

func (s *Service) writeComment(w http.ResponseWriter, r *http.Request, status int, id int64) error {
	c, err := s.q.GetComment(r.Context(), id)
	if err != nil {
		return err
	}
	viewer := auth.ActorFrom(r.Context())
	liked := false
	if viewer.Guest != nil {
		n, err := s.q.HasLiked(r.Context(), gen.HasLikedParams{CommentID: id, GuestID: viewer.Guest.ID})
		if err != nil {
			return err
		}
		liked = n > 0
	}
	httpx.WriteJSON(w, status, toJSON(row(c), viewer, liked))
	return nil
}

// loadEditable returns a live comment the viewer may change.
func (s *Service) loadEditable(r *http.Request) (row, error) {
	id, err := pathID(r)
	if err != nil {
		return row{}, err
	}
	ctx := r.Context()
	viewer := auth.ActorFrom(ctx)
	c, err := s.q.GetComment(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return row{}, httpx.ErrNotFound
	} else if err != nil {
		return row{}, err
	}
	if c.DeletedAt.Valid {
		return row{}, httpx.ErrNotFound
	}
	if err := s.visiblePost(ctx, c.PostID, viewer); err != nil {
		return row{}, err
	}
	if !canEdit(row(c), viewer) {
		return row{}, httpx.ErrForbidden
	}
	if !viewer.IsAdmin() && viewer.Guest != nil {
		if err := s.auth.CheckNotBanned(ctx, viewer.Guest.ID, viewer.Info.IPHash); err != nil {
			return row{}, err
		}
	}
	return row(c), nil
}

func (s *Service) handleUpdate(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		Body string `json:"body"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	c, err := s.loadEditable(r)
	if err != nil {
		return err
	}
	req.Body = strings.TrimSpace(req.Body)
	if e := checkBody(req.Body); e != nil {
		return e
	}
	if err := s.q.UpdateCommentBody(r.Context(), gen.UpdateCommentBodyParams{
		BodyMd: req.Body, BodyHtml: markdown.Render(req.Body), UpdatedAt: s.clock.Now().Unix(), ID: c.ID,
	}); err != nil {
		return err
	}
	return s.writeComment(w, r, http.StatusOK, c.ID)
}

func (s *Service) handleDelete(w http.ResponseWriter, r *http.Request) error {
	c, err := s.loadEditable(r)
	if err != nil {
		return err
	}
	viewer := auth.ActorFrom(r.Context())
	var by sql.NullInt64
	if viewer.IsAdmin() {
		by = sql.NullInt64{Int64: viewer.Admin.ID, Valid: true}
	}
	err = store.InTx(r.Context(), s.db, func(tx *sql.Tx, q *gen.Queries) error {
		if err := q.SoftDeleteComment(r.Context(), gen.SoftDeleteCommentParams{
			DeletedAt: sql.NullInt64{Int64: s.clock.Now().Unix(), Valid: true}, DeletedByAdminID: by, ID: c.ID,
		}); err != nil {
			return err
		}
		return q.RecountComments(r.Context(), c.PostID)
	})
	if err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}
