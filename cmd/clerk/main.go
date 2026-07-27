// Clerk writes standard project scaffolding, such as continuous
// integration configuration, into the current directory.
//
// Usage:
//
//	go tool clerk <type>
//
// The project types are:
//
//	app	a Go application
//	lib	a Go library
//
// Clerk records checksums of the files it writes to a clerk.sum
// file. On later runs, files it wrote before are updated in place,
// and clerk prompts before overwriting or deleting any file that
// changed since it was last written.
package main

import (
	"embed"
	"fmt"
	"io/fs"
	"os"

	"lesiw.io/clerk"
)

//go:embed all:app all:common all:lib
var files embed.FS

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "clerk:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: clerk <type>")
	}
	if args[0] != "app" && args[0] != "lib" {
		return fmt.Errorf("unknown project type %q", args[0])
	}
	var cfs clerk.ClerkFS
	for _, dir := range []string{"common", args[0]} {
		fsys, err := fs.Sub(files, dir)
		if err != nil {
			return fmt.Errorf("bad embedded fs: %w", err)
		}
		if err := cfs.Add(fsys); err != nil {
			return fmt.Errorf("add embedded fs: %w", err)
		}
	}
	if err := cfs.Apply("."); err != nil {
		return fmt.Errorf("apply %s: %w", args[0], err)
	}
	return nil
}
