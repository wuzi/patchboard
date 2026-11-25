package store_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"patchboard/internal/store"
)

func openTemp(t *testing.T) *sql.DB {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "pb.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := store.Migrate(context.Background(), db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func queryStrings(t *testing.T, db *sql.DB, q string) []string {
	t.Helper()
	rows, err := db.Query(q)
	if err != nil {
		t.Fatalf("%s: %v", q, err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		out = append(out, s)
	}
	return out
}

func queryInt(t *testing.T, db *sql.DB, q string, args ...any) int {
	t.Helper()
	var n int
	if err := db.QueryRow(q, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", q, err)
	}
	return n
}

func TestMigrate_FreshDBSeedsBoard(t *testing.T) {
	db := openTemp(t)
	got := queryStrings(t, db, "SELECT name_pt FROM statuses ORDER BY position")
	want := []string{"Sugestões", "Em análise", "Planejado", "Em desenvolvimento", "Concluído", "Recusado"}
	if len(got) != len(want) {
		t.Fatalf("statuses = %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("statuses = %v, want %v", got, want)
		}
	}
	if d := queryStrings(t, db, "SELECT name_pt FROM statuses WHERE is_default = 1"); len(d) != 1 || d[0] != "Sugestões" {
		t.Fatalf("default statuses = %v", d)
	}
	if n := queryInt(t, db, "SELECT count(*) FROM tags"); n != 3 {
		t.Fatalf("tags = %d", n)
	}
	if a := queryStrings(t, db, "SELECT accent_color FROM board_settings"); len(a) != 1 || a[0] != "#c026d3" {
		t.Fatalf("accent = %v", a)
	}
	if v := queryInt(t, db, "PRAGMA user_version"); v != 2 {
		t.Fatalf("user_version = %d", v)
	}
	if m := queryStrings(t, db, "PRAGMA journal_mode"); m[0] != "wal" {
		t.Fatalf("journal_mode = %v", m)
	}
	if fk := queryInt(t, db, "PRAGMA foreign_keys"); fk != 1 {
		t.Fatalf("foreign_keys = %d", fk)
	}
}

func TestMigrate_Idempotent(t *testing.T) {
	db := openTemp(t)
	if err := store.Migrate(context.Background(), db); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	if n := queryInt(t, db, "SELECT count(*) FROM statuses"); n != 6 {
		t.Fatalf("statuses = %d", n)
	}
}

func TestSchema_FTSTracksPostChanges(t *testing.T) {
	db := openTemp(t)
	mustExec := func(q string, args ...any) {
		t.Helper()
		if _, err := db.Exec(q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	mustExec(`INSERT INTO admins (id, username, display_name, password_hash, created_at) VALUES (1, 'a', 'A', 'x', 0)`)
	mustExec(`INSERT INTO posts (id, slug, title, status_id, author_admin_id, ip_hash, created_at, updated_at)
	          VALUES (1, 's', 'Melhorar análise de dano', 1, 1, 'h', 0, 0)`)
	match := func(q string) int {
		return queryInt(t, db, "SELECT count(*) FROM posts_fts WHERE posts_fts MATCH ?", q)
	}
	if n := match(`"analise"*`); n != 1 {
		t.Fatalf("analise matches = %d", n)
	}
	mustExec(`UPDATE posts SET title = 'Co-op' WHERE id = 1`)
	if n := match(`"analise"*`); n != 0 {
		t.Fatalf("stale match after update = %d", n)
	}
	if n := match(`"coop"*`); n != 0 {
		t.Fatalf("coop matches = %d", n)
	}
	if n := match(`"co"*`); n != 1 {
		t.Fatalf("co matches = %d", n)
	}
	mustExec(`DELETE FROM posts WHERE id = 1`)
	if n := match(`"co"*`); n != 0 {
		t.Fatalf("match after delete = %d", n)
	}
}

func TestMigrate_ConcurrentProcesses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pb.db")
	const n = 4
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		go func() {
			db, err := store.Open(path)
			if err != nil {
				errs <- err
				return
			}
			defer db.Close()
			errs <- store.Migrate(context.Background(), db)
		}()
	}
	for i := 0; i < n; i++ {
		if err := <-errs; err != nil {
			t.Fatalf("concurrent migrate: %v", err)
		}
	}
}
