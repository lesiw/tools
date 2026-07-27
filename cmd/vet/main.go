// Vet checks Go packages with the lesiw.io analyzer suite.
//
// Usage:
//
//	go tool lesiw.io/tools/cmd/vet [packages]
//
// Vet runs the go vet command with itself as the vet tool, so
// results are cached by the go build cache. Diagnostics are
// written to standard error, and vet exits with a non-zero status
// when any diagnostic is reported or analysis could not run.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"golang.org/x/tools/go/analysis/singlechecker"

	"lesiw.io/checker"
	"lesiw.io/command"
	"lesiw.io/command/sys"
)

var m = sys.Machine()
var executable = os.Executable

func main() {
	if os.Getenv("GOVET") == "1" {
		singlechecker.Main(checker.NewAnalyzer(analyzers...))
		return
	}
	if err := run(context.Background()); err != nil {
		cmdErr, ok := errors.AsType[*command.Error](err)
		if ok && cmdErr.Code != 0 {
			os.Exit(cmdErr.Code)
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	exe, err := executable()
	if err != nil {
		return fmt.Errorf("failed to get path to current executable: %w", err)
	}
	return command.Exec(
		command.WithEnv(ctx, map[string]string{"GOVET": "1"}), m,
		append([]string{"go", "vet", "-vettool=" + exe}, os.Args[1:]...)...,
	)
}
