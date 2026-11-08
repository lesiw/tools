// Command whitespace reports blank lines around declarations that do not
// follow the " + "layout rules.
package main

import (
	"golang.org/x/tools/go/analysis/singlechecker"

	"lesiw.io/tools/analyzers/linelen"
	"lesiw.io/tools/analyzers/whitespace"
)

// A command sets the width it runs with; the library default is no limit.
func init() { linelen.Config.Len = 79 }
func main() { singlechecker.Main(whitespace.Analyzer) }
