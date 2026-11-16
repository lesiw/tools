// Command redundantreturn reports a return with no results at the end of a
// function that has no results.
package main

import (
	"lesiw.io/tools/analyzers/redundantreturn"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(redundantreturn.Analyzer) }
