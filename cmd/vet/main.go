// Vet checks Go packages with the lesiw.io analyzer suite.
//
// Usage:
//
//	go tool lesiw.io/tools/cmd/vet [packages]
//
// Vet loads packages itself and reports diagnostics to standard
// error, exiting non-zero when any diagnostic is reported. The
// -fix flag applies the suite's suggested fixes.
package main

import (
	"golang.org/x/tools/go/analysis/singlechecker"

	"lesiw.io/checker"
)

func main() { singlechecker.Main(checker.NewAnalyzer(analyzers...)) }
