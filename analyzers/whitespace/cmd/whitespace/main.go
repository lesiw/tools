// Command whitespace reports blank lines around declarations that do not
// follow the layout rules.
package main

import (
	"lesiw.io/tools/analyzers/linelen"
	"lesiw.io/tools/analyzers/whitespace"
	"lesiw.io/tools/multichecker"
)

func init() { linelen.Config.Len = 79 }
func main() { multichecker.Main(whitespace.Analyzer) }
