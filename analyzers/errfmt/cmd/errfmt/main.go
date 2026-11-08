// Command errfmt reports error formatting that bypasses fmt.Errorf or %w.
package main

import (
	"golang.org/x/tools/go/analysis/singlechecker"

	"lesiw.io/tools/analyzers/errfmt"
)

func main() { singlechecker.Main(errfmt.Analyzer) }
