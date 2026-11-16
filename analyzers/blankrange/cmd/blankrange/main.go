// Command blankrange reports a range clause with an unnecessary blank
// identifier.
package main

import (
	"lesiw.io/tools/analyzers/blankrange"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(blankrange.Analyzer) }
