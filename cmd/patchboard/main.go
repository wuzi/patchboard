// Command patchboard runs the PatchBoard server and its admin commands.
package main

import (
	"fmt"
	"io"
	"os"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr, os.Getenv))
}

const usage = `usage:
  patchboard [serve]
  patchboard admin create --username NAME --name "Display Name"   (password on stdin)
  patchboard admin reset-password --username NAME                  (password on stdin)
`

func run(args []string, stdin io.Reader, stdout, stderr io.Writer, getenv func(string) string) int {
	if len(args) == 0 || args[0] == "serve" {
		if err := serve(getenv, stderr); err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return 1
		}
		return 0
	}
	fmt.Fprint(stderr, usage)
	return 2
}
