// Command lenzero reports a comparison of len with zero that is always the
// same.
package main

import (
	"lesiw.io/tools/analyzers/lenzero"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(lenzero.Analyzer) }
