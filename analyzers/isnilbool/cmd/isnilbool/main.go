// Command isnilbool reports a comparison of a comparison with a boolean
// literal.
package main

import (
	"lesiw.io/tools/analyzers/isnilbool"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(isnilbool.Analyzer) }
