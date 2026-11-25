package posts

import (
	"context"
	"database/sql"
	"net/http"
	"strconv"
	"strings"

	"patchboard/internal/api"
	"patchboard/internal/auth"
	"patchboard/internal/httpx"
)

const (
	defaultLimit = 30
	maxLimit     = 50
	roadmapCards = 100
	publicFilter = "p.deleted_at IS NULL AND p.merged_into_id IS NULL AND p.pending = 0"
	cardColumns  = `p.id, p.slug, p.title, p.excerpt, p.status_id, p.tag_id, p.vote_count, p.comment_count,
  p.pinned_at IS NOT NULL, p.pending, p.created_at,
  (SELECT count(*) FROM votes v WHERE v.post_id = p.id AND v.created_at >= ?) AS new_votes`
)

// ListQuery selects and orders posts for the List view.
type ListQuery struct {
	Sort            string
	StatusID, TagID *int64
	Q               string
	Offset, Limit   int
}

// filter is a WHERE clause fragment plus its args.
type filter struct {
	where []string
	args  []any
}

func (f *filter) add(cond string, args ...any) {
	f.where = append(f.where, cond)
	f.args = append(f.args, args...)
}

func (f *filter) sql() string { return strings.Join(f.where, " AND ") }

func publicFilterWith(statusID, tagID *int64, q string) filter {
	f := filter{}
	f.add(publicFilter)
	if statusID != nil {
		f.add("p.status_id = ?", *statusID)
	}
	if tagID != nil {
		f.add("p.tag_id = ?", *tagID)
	}
	if fts := FTSQuery(q, false); fts != "" {
		f.add("p.id IN (SELECT rowid FROM posts_fts WHERE posts_fts MATCH ?)", fts)
	} else if strings.TrimSpace(q) != "" {
		f.add("0") // a query with no searchable words matches nothing
	}
	return f
}

// queryCards runs a card query and fills voted/voters for the viewer.
func (s *Service) queryCards(ctx context.Context, viewer *auth.Actor, f filter, order string, limit, offset int) ([]api.PostCard, error) {
	query := "SELECT " + cardColumns + " FROM posts p WHERE " + f.sql() + " ORDER BY " + order + " LIMIT ? OFFSET ?"
	args := append([]any{s.newVotesSince()}, f.args...)
	args = append(args, limit, offset)
	rows, err := s.d.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cards := []api.PostCard{}
	for rows.Next() {
		var c api.PostCard
		var tag sql.NullInt64
		var pending int64
		var created int64
		if err := rows.Scan(&c.ID, &c.Slug, &c.Title, &c.Excerpt, &c.StatusID, &tag, &c.VoteCount, &c.CommentCount,
			&c.Pinned, &pending, &created, &c.NewVotes); err != nil {
			return nil, err
		}
		if tag.Valid {
			t := tag.Int64
			c.TagID = &t
		}
		c.Pending = pending != 0
		c.CreatedAt = ts(created)
		c.Voters = []api.Author{}
		cards = append(cards, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return cards, s.decorate(ctx, viewer, cards)
}

// decorate fills voted and the 3 most recent voters for each card.
func (s *Service) decorate(ctx context.Context, viewer *auth.Actor, cards []api.PostCard) error {
	if len(cards) == 0 {
		return nil
	}
	idx := map[int64]int{}
	ph := make([]string, len(cards))
	ids := make([]any, len(cards))
	for i, c := range cards {
		idx[c.ID] = i
		ph[i] = "?"
		ids[i] = c.ID
	}
	in := "(" + strings.Join(ph, ",") + ")"
	if viewer.Guest != nil {
		rows, err := s.d.DB.QueryContext(ctx, "SELECT post_id FROM votes WHERE guest_id = ? AND post_id IN "+in,
			append([]any{viewer.Guest.ID}, ids...)...)
		if err != nil {
			return err
		}
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
			cards[idx[id]].Voted = true
		}
		rows.Close()
	}
	rows, err := s.d.DB.QueryContext(ctx, `SELECT post_id, guest_id, name FROM (
  SELECT v.post_id, v.guest_id, g.name,
    ROW_NUMBER() OVER (PARTITION BY v.post_id ORDER BY v.created_at DESC, v.guest_id) AS rn
  FROM votes v JOIN guests g ON g.id = v.guest_id WHERE v.post_id IN `+in+`
) WHERE rn <= 3 ORDER BY post_id, rn`, ids...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var gid string
		var name sql.NullString
		if err := rows.Scan(&id, &gid, &name); err != nil {
			return err
		}
		i := idx[id]
		cards[i].Voters = append(cards[i].Voters, api.NewAuthor(sql.NullString{String: gid, Valid: true}, name, sql.NullInt64{}, sql.NullString{}))
	}
	return rows.Err()
}

// List returns one page of the List view.
func (s *Service) List(ctx context.Context, viewer *auth.Actor, q ListQuery) (api.PostPage, error) {
	if q.Limit <= 0 {
		q.Limit = defaultLimit
	}
	if q.Limit > maxLimit {
		q.Limit = maxLimit
	}
	pinned := "p.pinned_at IS NULL, "
	if strings.TrimSpace(q.Q) != "" {
		pinned = ""
	}
	var order string
	switch q.Sort {
	case "top":
		order = pinned + "p.vote_count DESC, p.id DESC"
	case "new":
		order = pinned + "p.created_at DESC, p.id DESC"
	default:
		order = pinned + "new_votes DESC, p.vote_count DESC, p.id DESC"
	}
	cards, err := s.queryCards(ctx, viewer, publicFilterWith(q.StatusID, q.TagID, q.Q), order, q.Limit+1, q.Offset)
	if err != nil {
		return api.PostPage{}, err
	}
	page := api.PostPage{Posts: cards}
	if len(cards) > q.Limit {
		page.Posts = cards[:q.Limit]
		next := q.Offset + q.Limit
		page.NextOffset = &next
	}
	return page, nil
}

// Roadmap returns one column per roadmap status with its top cards and total.
func (s *Service) Roadmap(ctx context.Context, viewer *auth.Actor, tagID *int64, q string) (api.Roadmap, error) {
	statuses, err := s.d.Board.Statuses(ctx)
	if err != nil {
		return api.Roadmap{}, err
	}
	rm := api.Roadmap{Columns: []api.RoadmapColumn{}}
	for _, st := range statuses {
		if !st.ShowOnRoadmap {
			continue
		}
		id := st.ID
		f := publicFilterWith(&id, tagID, q)
		cards, err := s.queryCards(ctx, viewer, f, "p.vote_count DESC, p.id DESC", roadmapCards, 0)
		if err != nil {
			return rm, err
		}
		var total int64
		if err := s.d.DB.QueryRowContext(ctx, "SELECT count(*) FROM posts p WHERE "+f.sql(), f.args...).Scan(&total); err != nil {
			return rm, err
		}
		rm.Columns = append(rm.Columns, api.RoadmapColumn{StatusID: st.ID, Total: total, Posts: cards})
	}
	return rm, nil
}

// optionalID parses an optional numeric query param into e on failure.
func optionalID(r *http.Request, name string, e *httpx.Error) *int64 {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return nil
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		e.Field(name, "invalid")
		return nil
	}
	return &v
}

func optionalInt(r *http.Request, name string, e *httpx.Error) int {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return 0
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < 0 {
		e.Field(name, "invalid")
		return 0
	}
	return v
}

func (s *Service) handleList(w http.ResponseWriter, r *http.Request) error {
	e := httpx.Validation()
	q := ListQuery{
		Sort:     r.URL.Query().Get("sort"),
		StatusID: optionalID(r, "status", e),
		TagID:    optionalID(r, "tag", e),
		Q:        r.URL.Query().Get("q"),
		Offset:   optionalInt(r, "offset", e),
		Limit:    optionalInt(r, "limit", e),
	}
	if len(e.Fields) > 0 {
		return e
	}
	page, err := s.List(r.Context(), auth.ActorFrom(r.Context()), q)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, page)
	return nil
}

func (s *Service) handleRoadmap(w http.ResponseWriter, r *http.Request) error {
	e := httpx.Validation()
	tag := optionalID(r, "tag", e)
	if len(e.Fields) > 0 {
		return e
	}
	rm, err := s.Roadmap(r.Context(), auth.ActorFrom(r.Context()), tag, r.URL.Query().Get("q"))
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, rm)
	return nil
}
