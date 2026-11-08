// Command whitespace reports blank lines around declarations that do not
// follow the " + "layout rules.
package main

import (
	"lesiw.io/tools/analyzers/linelen"
	"lesiw.io/tools/analyzers/whitespace"
	"lesiw.io/tools/singlechecker"
)

// A command sets the width it runs with; the library default is no limit.
func init() { linelen.Config.Len = 79 }
func main() { singlechecker.Main(whitespace.Analyzer) }
