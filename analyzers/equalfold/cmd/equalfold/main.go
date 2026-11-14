// Command equalfold reports a comparison of two strings or byte slices
// converted to the same case.
package main

import (
	"lesiw.io/tools/analyzers/equalfold"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(equalfold.Analyzer) }
