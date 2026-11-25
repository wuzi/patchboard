// Package store opens the SQLite database and applies migrations.
package store

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"patchboard/internal/store/gen"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// Open opens (creating if needed) the database at path with PatchBoard's pragmas.
func Open(path string) (*sql.DB, error) {
	// busy_timeout must come first so the WAL switch on a fresh file waits for other openers.
	dsn := "file:" + path +
		"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)" +
		"&_pragma=synchronous(NORMAL)&_txlock=immediate"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// Two processes switching a brand-new file to WAL at once can get SQLITE_BUSY without
	// the busy handler running; retry the first connection for up to ~5s.
	for attempt := 0; ; attempt++ {
		err = db.Ping()
		if err == nil {
			return db, nil
		}
		if attempt >= 50 || !strings.Contains(err.Error(), "SQLITE_BUSY") {
			db.Close()
			return nil, err
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// Migrate applies every embedded migration newer than PRAGMA user_version.
func Migrate(ctx context.Context, db *sql.DB) error {
	entries, err := fs.ReadDir(migrationFS, "migrations")
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)

	for _, name := range names {
		num, err := strconv.Atoi(strings.SplitN(name, "_", 2)[0])
		if err != nil {
			return fmt.Errorf("migration %s: bad number", name)
		}
		body, err := migrationFS.ReadFile("migrations/" + name)
		if err != nil {
			return err
		}
		if err := applyMigration(ctx, db, name, num, string(body)); err != nil {
			return err
		}
	}
	return nil
}

// applyMigration runs one migration unless user_version already covers it. The version is
// read inside the (immediate) write transaction, so concurrent processes cannot apply it twice.
func applyMigration(ctx context.Context, db *sql.DB, name string, num int, body string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var current int
	if err := tx.QueryRowContext(ctx, "PRAGMA user_version").Scan(&current); err != nil {
		return err
	}
	if num <= current {
		return nil
	}
	if _, err := tx.ExecContext(ctx, body); err != nil {
		return fmt.Errorf("migration %s: %w", name, err)
	}
	if _, err := tx.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", num)); err != nil {
		return err
	}
	return tx.Commit()
}

// InTx runs fn inside a write transaction, committing on success.
func InTx(ctx context.Context, db *sql.DB, fn func(tx *sql.Tx, q *gen.Queries) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx, gen.New(tx)); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}
