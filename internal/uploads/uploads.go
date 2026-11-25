package uploads

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base32"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"patchboard/internal/api"
	"patchboard/internal/auth"
	"patchboard/internal/board"
	"patchboard/internal/clock"
	"patchboard/internal/httpx"
	"patchboard/internal/store/gen"
)

const (
	maxFileBytes = 5 << 20
	maxPerPost   = 4
	orphanAge    = 24 * time.Hour
)

var nameRe = regexp.MustCompile(`^([a-z2-7]{20})\.(jpg|png)$`)

type Service struct {
	db      *sql.DB
	q       *gen.Queries
	clock   clock.Clock
	auth    *auth.Service
	board   *board.Service
	dataDir string
}

func NewService(db *sql.DB, c clock.Clock, a *auth.Service, b *board.Service, dataDir string) *Service {
	return &Service{db: db, q: gen.New(db), clock: c, auth: a, board: b, dataDir: dataDir}
}

func (s *Service) dir() string { return filepath.Join(s.dataDir, "uploads") }

func (s *Service) file(id, ext string) string { return filepath.Join(s.dir(), id+"."+ext) }

func imageJSON(u gen.Upload) api.Image {
	return api.Image{ID: u.ID, URL: "/uploads/" + u.ID + "." + u.Ext, Width: u.Width, Height: u.Height}
}

func newID() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b))
}

func tooLarge() *httpx.Error {
	return httpx.E(http.StatusRequestEntityTooLarge, "too_large", "images must be 5 MB or smaller")
}

func (s *Service) handleUpload(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	viewer := auth.ActorFrom(ctx)
	if !viewer.IsAdmin() {
		st, err := s.board.Settings(ctx)
		if err != nil {
			return err
		}
		if st.UploadsEnabled == 0 {
			return httpx.E(http.StatusForbidden, "uploads_disabled", "image uploads are turned off")
		}
	}
	if err := s.auth.Limit(r, httpx.RuleUpload); err != nil {
		return err
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxFileBytes+64<<10)
	mr, err := r.MultipartReader()
	if err != nil {
		return httpx.E(http.StatusBadRequest, "bad_request", "expected multipart/form-data")
	}
	var raw []byte
	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			var mbe *http.MaxBytesError
			if errors.As(err, &mbe) {
				return tooLarge()
			}
			return httpx.E(http.StatusBadRequest, "bad_request", "invalid multipart body")
		}
		if part.FormName() != "file" {
			continue
		}
		raw, err = io.ReadAll(io.LimitReader(part, maxFileBytes+1))
		if err != nil {
			var mbe *http.MaxBytesError
			if errors.As(err, &mbe) {
				return tooLarge()
			}
			return err
		}
		break
	}
	if raw == nil {
		return httpx.Validation().Field("file", "required")
	}
	if len(raw) > maxFileBytes {
		return tooLarge()
	}
	data, ext, width, height, err := Process(bytes.NewReader(raw))
	switch {
	case errors.Is(err, ErrTooManyPixels):
		return httpx.E(http.StatusUnprocessableEntity, "too_many_pixels", "image dimensions are too large")
	case errors.Is(err, ErrNotImage):
		return httpx.E(http.StatusUnprocessableEntity, "invalid_image", "file is not a JPEG, PNG, GIF or WebP image")
	case err != nil:
		return err
	}
	author, err := s.auth.Writer(w, r)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.dir(), 0o700); err != nil {
		return err
	}
	id := newID()
	if err := os.WriteFile(s.file(id, ext), data, 0o600); err != nil {
		return err
	}
	u := gen.InsertUploadParams{
		ID: id, Ext: ext, Width: int64(width), Height: int64(height), Bytes: int64(len(data)),
		IpHash: viewer.Info.IPHash, CreatedAt: s.clock.Now().Unix(),
	}
	if author.GuestID != nil {
		u.GuestID = sql.NullString{String: *author.GuestID, Valid: true}
	} else {
		u.AdminID = sql.NullInt64{Int64: *author.AdminID, Valid: true}
	}
	if err := s.q.InsertUpload(ctx, u); err != nil {
		os.Remove(s.file(id, ext))
		return err
	}
	httpx.WriteJSON(w, http.StatusCreated, api.Image{ID: id, URL: "/uploads/" + id + "." + ext, Width: int64(width), Height: int64(height)})
	return nil
}

// ForPost lists a post's images in order.
func (s *Service) ForPost(ctx context.Context, postID int64) ([]api.Image, error) {
	rows, err := s.q.ListPostUploads(ctx, sql.NullInt64{Int64: postID, Valid: true})
	if err != nil {
		return nil, err
	}
	out := make([]api.Image, 0, len(rows))
	for _, u := range rows {
		out = append(out, imageJSON(u))
	}
	return out, nil
}

// Attach makes ids (in order) the post's images; others are detached and later cleaned up.
func (s *Service) Attach(ctx context.Context, tx *sql.Tx, postID int64, ids []string, owner auth.AuthorRef) error {
	if len(ids) > maxPerPost {
		return httpx.Validation().Field("imageIds", "too_many")
	}
	q := gen.New(tx)
	seen := map[string]bool{}
	for _, id := range ids {
		u, err := q.GetUpload(ctx, id)
		if errors.Is(err, sql.ErrNoRows) {
			return httpx.Validation().Field("imageIds", "invalid")
		} else if err != nil {
			return err
		}
		owned := (owner.GuestID != nil && u.GuestID.Valid && u.GuestID.String == *owner.GuestID) ||
			(owner.AdminID != nil && u.AdminID.Valid && u.AdminID.Int64 == *owner.AdminID)
		free := !u.PostID.Valid || u.PostID.Int64 == postID
		if !owned || !free || seen[id] {
			return httpx.Validation().Field("imageIds", "invalid")
		}
		seen[id] = true
	}
	pid := sql.NullInt64{Int64: postID, Valid: true}
	if err := q.DetachPostUploads(ctx, pid); err != nil {
		return err
	}
	for i, id := range ids {
		if err := q.AttachUpload(ctx, gen.AttachUploadParams{PostID: pid, Position: int64(i), ID: id}); err != nil {
			return err
		}
	}
	return nil
}

// CleanupOrphans deletes uploads never attached (or detached) for over 24 hours, except the logo.
func (s *Service) CleanupOrphans(ctx context.Context) (int, error) {
	rows, err := s.q.ListOrphanUploads(ctx, s.clock.Now().Add(-orphanAge).Unix())
	if err != nil {
		return 0, err
	}
	for _, u := range rows {
		if err := s.remove(ctx, u.ID, u.Ext); err != nil {
			return 0, err
		}
	}
	return len(rows), nil
}

func (s *Service) remove(ctx context.Context, id, ext string) error {
	if err := s.q.DeleteUpload(ctx, id); err != nil {
		return err
	}
	if err := os.Remove(s.file(id, ext)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (s *Service) handleServe(w http.ResponseWriter, r *http.Request) {
	m := nameRe.FindStringSubmatch(r.PathValue("name"))
	if m == nil {
		http.NotFound(w, r)
		return
	}
	u, err := s.q.GetUpload(r.Context(), m[1])
	if err != nil || u.Ext != m[2] {
		http.NotFound(w, r)
		return
	}
	f, err := os.Open(s.file(u.ID, u.Ext))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	ct := "image/jpeg"
	if u.Ext == "png" {
		ct = "image/png"
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, r, "", time.Unix(u.CreatedAt, 0), f)
}

func (s *Service) handleAdminDelete(w http.ResponseWriter, r *http.Request) error {
	u, err := s.q.GetUpload(r.Context(), r.PathValue("id"))
	if errors.Is(err, sql.ErrNoRows) {
		return httpx.ErrNotFound
	} else if err != nil {
		return err
	}
	if err := s.remove(r.Context(), u.ID, u.Ext); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// Register adds the upload routes.
func (s *Service) Register(mux *http.ServeMux, requireAdmin func(http.Handler) http.Handler) {
	mux.Handle("POST /api/uploads", httpx.HandlerFunc(s.handleUpload))
	mux.Handle("GET /uploads/{name}", http.HandlerFunc(s.handleServe))
	mux.Handle("DELETE /api/admin/uploads/{id}", requireAdmin(httpx.HandlerFunc(s.handleAdminDelete)))
}
