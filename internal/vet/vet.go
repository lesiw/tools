// Package vet runs the lesiw.io analyzer suite over Go packages.
package vet

import (
	"os"

	"golang.org/x/tools/go/analysis/singlechecker"

	"lesiw.io/checker"
)

// Main runs the analysis driver with args. It does not return. The driver
// exits with the status the run earned.
//
// The driver parses os.Args, not its arguments, so the arguments go there.
func Main(args []string) {
	os.Args = append([]string{"vet"}, args...)
	singlechecker.Main(checker.NewAnalyzer(analyzers...))
}
