package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/term"

	"patchboard/internal/auth"
	"patchboard/internal/config"
	"patchboard/internal/httpx"
	"patchboard/internal/store"
)

func adminCmd(args []string, stdin io.Reader, stdout, stderr io.Writer, getenv func(string) string) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	fs := flag.NewFlagSet("admin "+args[0], flag.ContinueOnError)
	fs.SetOutput(stderr)
	username := fs.String("username", "", "admin username")
	name := fs.String("name", "", "display name")
	if err := fs.Parse(args[1:]); err != nil {
		fmt.Fprint(stderr, usage)
		return 2
	}
	switch args[0] {
	case "create":
		if *username == "" || *name == "" {
			fmt.Fprint(stderr, usage)
			return 2
		}
	case "reset-password":
		if *username == "" {
			fmt.Fprint(stderr, usage)
			return 2
		}
	default:
		fmt.Fprint(stderr, usage)
		return 2
	}

	cfg, err := config.Load(getenv)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	db, err := store.Open(filepath.Join(cfg.DataDir, "patchboard.db"))
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	defer db.Close()
	ctx := context.Background()
	if err := store.Migrate(ctx, db); err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	pw, err := readPassword(stdin, stderr)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	if args[0] == "create" {
		_, err = auth.CreateAdmin(ctx, db, time.Now().UTC(), *username, *name, pw)
	} else {
		err = auth.ResetPassword(ctx, db, *username, pw)
	}
	if err != nil {
		fmt.Fprintln(stderr, "error:", describe(err))
		return 1
	}
	fmt.Fprintf(stdout, "admin %q: done\n", strings.ToLower(*username))
	return 0
}

// readPassword prompts twice on a terminal, otherwise reads one line.
func readPassword(stdin io.Reader, stderr io.Writer) (string, error) {
	if f, ok := stdin.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		fmt.Fprint(stderr, "Password: ")
		a, err := term.ReadPassword(int(f.Fd()))
		fmt.Fprintln(stderr)
		if err != nil {
			return "", err
		}
		fmt.Fprint(stderr, "Repeat password: ")
		b, err := term.ReadPassword(int(f.Fd()))
		fmt.Fprintln(stderr)
		if err != nil {
			return "", err
		}
		if string(a) != string(b) {
			return "", errors.New("passwords do not match")
		}
		return string(a), nil
	}
	line, err := bufio.NewReader(stdin).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

func describe(err error) string {
	var e *httpx.Error
	if errors.As(err, &e) && len(e.Fields) > 0 {
		parts := []string{}
		for f, reason := range e.Fields {
			parts = append(parts, f+" "+reason)
		}
		return e.Message + " (" + strings.Join(parts, ", ") + ")"
	}
	return err.Error()
}
