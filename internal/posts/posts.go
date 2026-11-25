package posts

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
	"patchboard/internal/board"
	"patchboard/internal/clock"
	"patchboard/internal/httpx"
	"patchboard/internal/markdown"
	"patchboard/internal/store"
	"patchboard/internal/store/gen"
)

const (
	minTitle   = 3
	maxTitle   = 120
	excerptLen = 140
	maxMerge   = 20
)

type (
	CommentLoader func(ctx context.Context, postID int64, viewer *auth.Actor) ([]api.Comment, error)
	ImageStore    interface {
		ForPost(ctx context.Context, postID int64) ([]api.Image, error)
		Attach(ctx context.Context, tx *sql.Tx, postID int64, ids []string, owner auth.AuthorRef) error
	}
	UpdateCreator func(ctx context.Context, tx *sql.Tx, adminID int64, title, body string, postIDs []int64) (int64, error)
)

type Deps struct {
	DB           *sql.DB
	Clock        clock.Clock
	Auth         *auth.Service
	Board        *board.Service
	Comments     CommentLoader
	Images       ImageStore
	CreateUpdate UpdateCreator
}

type Service struct {
	d Deps
	q *gen.Queries
}

func NewService(d Deps) *Service { return &Service{d: d, q: gen.New(d.DB)} }

func ts(unix int64) time.Time { return time.Unix(unix, 0).UTC() }

func nullInt(v *int64) sql.NullInt64 {
	if v == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *v, Valid: true}
}

func nullStr(v *string) sql.NullString {
	if v == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *v, Valid: true}
}

func (s *Service) validate(ctx context.Context, e *httpx.Error, title, body *string, tagID *int64) error {
	if title != nil {
		*title = strings.TrimSpace(*title)
		switch n := utf8.RuneCountInString(*title); {
		case n < minTitle:
			e.Field("title", "too_short")
		case n > maxTitle:
			e.Field("title", "too_long")
		}
	}
	if body != nil {
		*body = strings.TrimSpace(*body)
		if utf8.RuneCountInString(*body) > markdown.MaxBody {
			e.Field("body", "too_long")
		}
	}
	if tagID != nil {
		ok, err := s.d.Board.TagExists(ctx, *tagID)
		if err != nil {
			return err
		}
		if !ok {
			e.Field("tagId", "invalid")
		}
	}
	return nil
}

type createReq struct {
	Title    string   `json:"title"`
	Body     string   `json:"body"`
	TagID    *int64   `json:"tagId"`
	ImageIDs []string `json:"imageIds"`
}

func (s *Service) handleCreate(w http.ResponseWriter, r *http.Request) error {
	var req createReq
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	ctx := r.Context()
	e := httpx.Validation()
	if err := s.validate(ctx, e, &req.Title, &req.Body, req.TagID); err != nil {
		return err
	}
	if len(e.Fields) > 0 {
		return e
	}
	if err := s.d.Auth.Limit(r, httpx.RulePostCreate); err != nil {
		return err
	}
	author, err := s.d.Auth.Writer(w, r)
	if err != nil {
		return err
	}
	settings, err := s.d.Board.Settings(ctx)
	if err != nil {
		return err
	}
	status, err := s.d.Board.DefaultStatusID(ctx)
	if err != nil {
		return err
	}
	pending := int64(0)
	if settings.RequireApproval != 0 && author.AdminID == nil {
		pending = 1
	}
	now := s.d.Clock.Now().Unix()
	var id int64
	err = store.InTx(ctx, s.d.DB, func(tx *sql.Tx, q *gen.Queries) error {
		id, err = q.InsertPost(ctx, gen.InsertPostParams{
			Slug: Slugify(req.Title), Title: req.Title, BodyMd: req.Body, BodyHtml: markdown.Render(req.Body),
			Excerpt: markdown.Excerpt(req.Body, excerptLen), StatusID: status, TagID: nullInt(req.TagID),
			AuthorGuestID: nullStr(author.GuestID), AuthorAdminID: nullInt(author.AdminID),
			IpHash: auth.ActorFrom(ctx).Info.IPHash, Pending: pending, CreatedAt: now, UpdatedAt: now,
		})
		if err != nil {
			return err
		}
		if len(req.ImageIDs) > 0 {
			return s.d.Images.Attach(ctx, tx, id, req.ImageIDs, author)
		}
		return nil
	})
	if err != nil {
		return err
	}
	res, err := s.Detail(ctx, id, auth.ActorFrom(ctx))
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusCreated, res.Post)
	return nil
}

// load fetches a post row; sql.ErrNoRows becomes 404.
func (s *Service) load(ctx context.Context, id int64) (postRow, error) {
	p, err := s.q.GetPostFull(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return p, httpx.ErrNotFound
	}
	return p, err
}

// resolveMerge follows merged_into_id to the final post.
func (s *Service) resolveMerge(ctx context.Context, id int64) (*api.PostRef, error) {
	for i := 0; i < maxMerge; i++ {
		ref, err := s.q.GetPostRef(ctx, id)
		if err != nil {
			return nil, err
		}
		if !ref.MergedIntoID.Valid {
			return &api.PostRef{ID: ref.ID, Slug: ref.Slug}, nil
		}
		id = ref.MergedIntoID.Int64
	}
	return nil, errors.New("merge chain too long")
}

// Detail returns the post as the viewer may see it (404 when hidden).
func (s *Service) Detail(ctx context.Context, id int64, viewer *auth.Actor) (api.PostResponse, error) {
	p, err := s.load(ctx, id)
	if err != nil {
		return api.PostResponse{}, err
	}
	if p.MergedIntoID.Valid {
		ref, err := s.resolveMerge(ctx, p.MergedIntoID.Int64)
		if err != nil {
			return api.PostResponse{}, err
		}
		return api.PostResponse{MergedInto: ref}, nil
	}
	if !visible(p, viewer) {
		return api.PostResponse{}, httpx.ErrNotFound
	}
	d, err := s.detail(ctx, p, viewer)
	if err != nil {
		return api.PostResponse{}, err
	}
	return api.PostResponse{Post: &d}, nil
}

func (s *Service) detail(ctx context.Context, p postRow, viewer *auth.Actor) (api.PostDetail, error) {
	defaultStatus, err := s.d.Board.DefaultStatusID(ctx)
	if err != nil {
		return api.PostDetail{}, err
	}
	privileged := viewer.IsAdmin() || isAuthor(p, viewer)
	d := api.PostDetail{
		PostCard: api.PostCard{
			ID: p.ID, Slug: p.Slug, Title: p.Title, Excerpt: p.Excerpt, StatusID: p.StatusID,
			VoteCount: p.VoteCount, CommentCount: p.CommentCount, Voters: []api.Author{},
			Pinned: p.PinnedAt.Valid, Pending: p.Pending != 0, CreatedAt: ts(p.CreatedAt),
		},
		BodyHTML:       p.BodyHtml,
		Author:         api.NewAuthor(p.AuthorGuestID, p.GuestName, p.AuthorAdminID, sql.NullString{String: p.AdminName, Valid: p.AuthorAdminID.Valid}),
		CommentsLocked: p.CommentsLocked != 0,
		Deleted:        p.DeletedAt.Valid,
		CanEdit:        canEdit(p, viewer, defaultStatus),
	}
	d.CanDelete = d.CanEdit && !d.Deleted
	if p.TagID.Valid {
		t := p.TagID.Int64
		d.TagID = &t
	}
	if privileged {
		d.BodyMD = p.BodyMd
	}
	if p.ResponseHtml.Valid {
		d.Response = &api.TeamResponse{
			HTML: p.ResponseHtml.String,
			Author: api.NewAuthor(sql.NullString{}, sql.NullString{}, p.ResponseAdminID,
				sql.NullString{String: p.ResponseAdminName, Valid: true}),
			At: ts(p.ResponseAt.Int64),
		}
		if privileged {
			d.Response.MD = p.ResponseMd.String
		}
	}
	if viewer.Guest != nil {
		n, err := s.q.HasVoted(ctx, gen.HasVotedParams{PostID: p.ID, GuestID: viewer.Guest.ID})
		if err != nil {
			return d, err
		}
		d.Voted = n > 0
	}
	voters, err := s.q.RecentVoters(ctx, p.ID)
	if err != nil {
		return d, err
	}
	for _, v := range voters {
		d.Voters = append(d.Voters, api.NewAuthor(sql.NullString{String: v.GuestID, Valid: true}, v.Name, sql.NullInt64{}, sql.NullString{}))
	}
	if d.NewVotes, err = s.q.CountVotesSince(ctx, gen.CountVotesSinceParams{PostID: p.ID, CreatedAt: s.newVotesSince()}); err != nil {
		return d, err
	}
	if d.Images, err = s.d.Images.ForPost(ctx, p.ID); err != nil {
		return d, err
	}
	if d.Comments, err = s.d.Comments(ctx, p.ID, viewer); err != nil {
		return d, err
	}
	if d.Images == nil {
		d.Images = []api.Image{}
	}
	if d.Comments == nil {
		d.Comments = []api.Comment{}
	}
	return d, nil
}

// newVotesSince is the start of the "this week" window.
func (s *Service) newVotesSince() int64 {
	return s.d.Clock.Now().Add(-7 * 24 * time.Hour).Unix()
}

func (s *Service) handleGet(w http.ResponseWriter, r *http.Request) error {
	id, err := postID(r)
	if err != nil {
		return err
	}
	res, err := s.Detail(r.Context(), id, auth.ActorFrom(r.Context()))
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, res)
	return nil
}

// editable loads a post the viewer may modify, or returns 404/403.
func (s *Service) editable(ctx context.Context, id int64, viewer *auth.Actor) (postRow, error) {
	p, err := s.load(ctx, id)
	if err != nil {
		return p, err
	}
	if p.MergedIntoID.Valid || !visible(p, viewer) {
		return p, httpx.ErrNotFound
	}
	def, err := s.d.Board.DefaultStatusID(ctx)
	if err != nil {
		return p, err
	}
	if canEdit(p, viewer, def) {
		return p, nil
	}
	if isAuthor(p, viewer) {
		return p, httpx.E(http.StatusForbidden, "edit_window_closed", "the team is already working on this post")
	}
	return p, httpx.ErrForbidden
}

func (s *Service) handleUpdate(w http.ResponseWriter, r *http.Request) error {
	id, err := postID(r)
	if err != nil {
		return err
	}
	var req struct {
		Title    *string   `json:"title"`
		Body     *string   `json:"body"`
		TagID    *int64    `json:"tagId"`
		ClearTag bool      `json:"clearTag"`
		ImageIDs *[]string `json:"imageIds"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	ctx := r.Context()
	viewer := auth.ActorFrom(ctx)
	p, err := s.editable(ctx, id, viewer)
	if err != nil {
		return err
	}
	e := httpx.Validation()
	if err := s.validate(ctx, e, req.Title, req.Body, req.TagID); err != nil {
		return err
	}
	if len(e.Fields) > 0 {
		return e
	}
	title, body, tag := p.Title, p.BodyMd, p.TagID
	if req.Title != nil {
		title = *req.Title
	}
	if req.Body != nil {
		body = *req.Body
	}
	if req.TagID != nil {
		tag = nullInt(req.TagID)
	}
	if req.ClearTag {
		tag = sql.NullInt64{}
	}
	owner := auth.AuthorRef{}
	if p.AuthorAdminID.Valid {
		owner.AdminID = &p.AuthorAdminID.Int64
	} else {
		owner.GuestID = &p.AuthorGuestID.String
	}
	err = store.InTx(ctx, s.d.DB, func(tx *sql.Tx, q *gen.Queries) error {
		if err := q.UpdatePostContent(ctx, gen.UpdatePostContentParams{
			Slug: Slugify(title), Title: title, BodyMd: body, BodyHtml: markdown.Render(body),
			Excerpt: markdown.Excerpt(body, excerptLen), TagID: tag, UpdatedAt: s.d.Clock.Now().Unix(), ID: id,
		}); err != nil {
			return err
		}
		if req.ImageIDs != nil {
			return s.d.Images.Attach(ctx, tx, id, *req.ImageIDs, owner)
		}
		return nil
	})
	if err != nil {
		return err
	}
	res, err := s.Detail(ctx, id, viewer)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, res.Post)
	return nil
}

func (s *Service) handleDelete(w http.ResponseWriter, r *http.Request) error {
	id, err := postID(r)
	if err != nil {
		return err
	}
	ctx := r.Context()
	viewer := auth.ActorFrom(ctx)
	p, err := s.editable(ctx, id, viewer)
	if err != nil {
		return err
	}
	if !p.DeletedAt.Valid {
		var by sql.NullInt64
		if viewer.IsAdmin() {
			by = sql.NullInt64{Int64: viewer.Admin.ID, Valid: true}
		}
		if err := s.q.SoftDeletePost(ctx, gen.SoftDeletePostParams{
			DeletedAt: sql.NullInt64{Int64: s.d.Clock.Now().Unix(), Valid: true}, DeletedByAdminID: by, ID: id,
		}); err != nil {
			return err
		}
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}
