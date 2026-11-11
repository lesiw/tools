// Command errfmt reports error formatting that bypasses fmt.Errorf or %w.
package main

import (
	"lesiw.io/tools/analyzers/errfmt"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(errfmt.Analyzer) }
