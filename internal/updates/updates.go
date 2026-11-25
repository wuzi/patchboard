// Package updates owns dated announcements linked to posts.
package updates

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"patchboard/internal/api"
	"patchboard/internal/auth"
	"patchboard/internal/clock"
	"patchboard/internal/httpx"
	"patchboard/internal/markdown"
	"patchboard/internal/store"
	"patchboard/internal/store/gen"
)

const (
	maxTitle = 120
	maxLinks = 10
	pageSize = 20
)

type Service struct {
	db    *sql.DB
	q     *gen.Queries
	clock clock.Clock
}

func NewService(db *sql.DB, c clock.Clock) *Service {
	return &Service{db: db, q: gen.New(db), clock: c}
}

// validate trims and checks title/body/postIds, recording field errors in e.
func validate(ctx context.Context, q *gen.Queries, e *httpx.Error, title, body *string, postIDs []int64) error {
	if title != nil {
		*title = strings.TrimSpace(*title)
		switch n := utf8.RuneCountInString(*title); {
		case n == 0:
			e.Field("title", "required")
		case n > maxTitle:
			e.Field("title", "too_long")
		}
	}
	if body != nil {
		*body = strings.TrimSpace(*body)
		switch n := utf8.RuneCountInString(*body); {
		case n == 0:
			e.Field("body", "required")
		case n > markdown.MaxBody:
			e.Field("body", "too_long")
		}
	}
	if len(postIDs) > maxLinks {
		e.Field("postIds", "too_many")
		return nil
	}
	seen := map[int64]bool{}
	for _, id := range postIDs {
		p, err := q.GetPostForLink(ctx, id)
		if errors.Is(err, sql.ErrNoRows) || (err == nil && p.DeletedAt.Valid) || seen[id] {
			e.Field("postIds", "invalid")
			return nil
		} else if err != nil {
			return err
		}
		seen[id] = true
	}
	return nil
}

// setLinks replaces an update's linked posts, keeping existing status snapshots.
func setLinks(ctx context.Context, q *gen.Queries, updateID int64, postIDs []int64) error {
	old, err := q.ListUpdateLinks(ctx, updateID)
	if err != nil {
		return err
	}
	snap := map[int64]int64{}
	for _, l := range old {
		snap[l.PostID] = l.StatusID
	}
	if err := q.DeleteUpdateLinks(ctx, updateID); err != nil {
		return err
	}
	for i, id := range postIDs {
		status, ok := snap[id]
		if !ok {
			p, err := q.GetPostForLink(ctx, id)
			if err != nil {
				return err
			}
			status = p.StatusID
		}
		if err := q.InsertUpdatePost(ctx, gen.InsertUpdatePostParams{UpdateID: updateID, PostID: id, StatusID: status, Position: int64(i)}); err != nil {
			return err
		}
	}
	return nil
}

// CreateTx validates and inserts an Update inside tx (used by status changes too).
func (s *Service) CreateTx(ctx context.Context, tx *sql.Tx, adminID int64, title, body string, postIDs []int64) (int64, error) {
	q := gen.New(tx)
	e := httpx.Validation()
	if err := validate(ctx, q, e, &title, &body, postIDs); err != nil {
		return 0, err
	}
	if len(e.Fields) > 0 {
		return 0, e
	}
	now := s.clock.Now().Unix()
	id, err := q.InsertUpdate(ctx, gen.InsertUpdateParams{
		Title: title, BodyMd: body, BodyHtml: markdown.Render(body), AuthorAdminID: adminID, PublishedAt: now, UpdatedAt: now,
	})
	if err != nil {
		return 0, err
	}
	return id, setLinks(ctx, q, id, postIDs)
}

type updateRow = gen.GetUpdateRow

func (s *Service) item(ctx context.Context, u updateRow, viewer *auth.Actor) (api.UpdateItem, error) {
	it := api.UpdateItem{
		ID: u.ID, Title: u.Title, BodyHTML: u.BodyHtml, PublishedAt: time.Unix(u.PublishedAt, 0).UTC(),
		Author: api.NewAuthor(sql.NullString{}, sql.NullString{}, sql.NullInt64{Int64: u.AuthorAdminID, Valid: true}, sql.NullString{String: u.AdminName, Valid: true}),
		Posts:  []api.UpdatePost{},
	}
	if viewer.IsAdmin() {
		it.BodyMD = u.BodyMd
	}
	links, err := s.q.ListPublicUpdatePosts(ctx, u.ID)
	if err != nil {
		return it, err
	}
	for _, l := range links {
		it.Posts = append(it.Posts, api.UpdatePost{ID: l.ID, Slug: l.Slug, Title: l.Title, StatusID: l.StatusID})
	}
	return it, nil
}

func (s *Service) writeItem(w http.ResponseWriter, r *http.Request, status int, id int64) error {
	u, err := s.q.GetUpdate(r.Context(), id)
	if err != nil {
		return err
	}
	it, err := s.item(r.Context(), u, auth.ActorFrom(r.Context()))
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, status, it)
	return nil
}

type body struct {
	Title   *string  `json:"title"`
	Body    *string  `json:"body"`
	PostIDs *[]int64 `json:"postIds"`
}

func (s *Service) handleCreate(w http.ResponseWriter, r *http.Request) error {
	var req body
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	title, text, ids := "", "", []int64{}
	if req.Title != nil {
		title = *req.Title
	}
	if req.Body != nil {
		text = *req.Body
	}
	if req.PostIDs != nil {
		ids = *req.PostIDs
	}
	var id int64
	err := store.InTx(r.Context(), s.db, func(tx *sql.Tx, _ *gen.Queries) error {
		var err error
		id, err = s.CreateTx(r.Context(), tx, auth.ActorFrom(r.Context()).Admin.ID, title, text, ids)
		return err
	})
	if err != nil {
		return err
	}
	return s.writeItem(w, r, http.StatusCreated, id)
}

func pathID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		return 0, httpx.ErrNotFound
	}
	return id, nil
}

func (s *Service) handleEdit(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	var req body
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	ctx := r.Context()
	err = store.InTx(ctx, s.db, func(tx *sql.Tx, q *gen.Queries) error {
		cur, err := q.GetUpdate(ctx, id)
		if errors.Is(err, sql.ErrNoRows) {
			return httpx.ErrNotFound
		} else if err != nil {
			return err
		}
		var ids []int64
		if req.PostIDs != nil {
			ids = *req.PostIDs
		}
		e := httpx.Validation()
		if err := validate(ctx, q, e, req.Title, req.Body, ids); err != nil {
			return err
		}
		if len(e.Fields) > 0 {
			return e
		}
		title, text := cur.Title, cur.BodyMd
		if req.Title != nil {
			title = *req.Title
		}
		if req.Body != nil {
			text = *req.Body
		}
		if err := q.EditUpdate(ctx, gen.EditUpdateParams{
			Title: title, BodyMd: text, BodyHtml: markdown.Render(text), UpdatedAt: s.clock.Now().Unix(), ID: id,
		}); err != nil {
			return err
		}
		if req.PostIDs != nil {
			return setLinks(ctx, q, id, ids)
		}
		return nil
	})
	if err != nil {
		return err
	}
	return s.writeItem(w, r, http.StatusOK, id)
}

func (s *Service) handleDelete(w http.ResponseWriter, r *http.Request) error {
	id, err := pathID(r)
	if err != nil {
		return err
	}
	if _, err := s.q.GetUpdate(r.Context(), id); errors.Is(err, sql.ErrNoRows) {
		return httpx.ErrNotFound
	} else if err != nil {
		return err
	}
	if err := s.q.DeleteUpdate(r.Context(), id); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Service) handleList(w http.ResponseWriter, r *http.Request) error {
	offset := 0
	if raw := r.URL.Query().Get("offset"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v < 0 {
			return httpx.Validation().Field("offset", "invalid")
		}
		offset = v
	}
	rows, err := s.q.ListUpdates(r.Context(), gen.ListUpdatesParams{Limit: pageSize + 1, Offset: int64(offset)})
	if err != nil {
		return err
	}
	page := api.UpdatePage{Updates: []api.UpdateItem{}}
	viewer := auth.ActorFrom(r.Context())
	for i, u := range rows {
		if i == pageSize {
			next := offset + pageSize
			page.NextOffset = &next
			break
		}
		it, err := s.item(r.Context(), updateRow(u), viewer)
		if err != nil {
			return err
		}
		page.Updates = append(page.Updates, it)
	}
	httpx.WriteJSON(w, http.StatusOK, page)
	return nil
}

// Register adds the Updates routes.
func (s *Service) Register(mux *http.ServeMux, requireAdmin func(http.Handler) http.Handler) {
	mux.Handle("GET /api/updates", httpx.HandlerFunc(s.handleList))
	mux.Handle("POST /api/admin/updates", requireAdmin(httpx.HandlerFunc(s.handleCreate)))
	mux.Handle("PATCH /api/admin/updates/{id}", requireAdmin(httpx.HandlerFunc(s.handleEdit)))
	mux.Handle("DELETE /api/admin/updates/{id}", requireAdmin(httpx.HandlerFunc(s.handleDelete)))
}
