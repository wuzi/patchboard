package main

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"patchboard/internal/auth"
	"patchboard/internal/store"
)

func TestCLI_AdminCreateAndReset(t *testing.T) {
	dir := t.TempDir()
	env := func(k string) string {
		return map[string]string{"PB_BASE_URL": "http://localhost:8080", "PB_DATA_DIR": dir}[k]
	}
	var out, errb bytes.Buffer
	code := run([]string{"admin", "create", "--username", "wuzi", "--name", "Wuzi"}, strings.NewReader("supersecret1\n"), &out, &errb, env)
	if code != 0 {
		t.Fatalf("create exit %d: %s", code, errb.String())
	}
	code = run([]string{"admin", "reset-password", "--username", "wuzi"}, strings.NewReader("anothersecret\n"), &out, &errb, env)
	if code != 0 {
		t.Fatalf("reset exit %d: %s", code, errb.String())
	}
	db, err := store.Open(filepath.Join(dir, "patchboard.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var hash string
	if err := db.QueryRowContext(context.Background(), `SELECT password_hash FROM admins WHERE username = 'wuzi'`).Scan(&hash); err != nil {
		t.Fatal(err)
	}
	if ok, _ := auth.VerifyPassword(hash, "anothersecret"); !ok {
		t.Fatal("password was not reset")
	}
	errb.Reset()
	if code := run([]string{"admin", "create", "--username", "x"}, strings.NewReader(""), &out, &errb, env); code != 2 {
		t.Fatalf("missing --name exit = %d", code)
	}
	if !strings.Contains(errb.String(), "usage") {
		t.Fatalf("stderr = %q", errb.String())
	}
}
