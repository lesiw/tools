// Command bookends reports a closing bracket that is not at the indent of its
// opening bracket.
package main

import (
	"golang.org/x/tools/go/analysis/singlechecker"

	"lesiw.io/tools/analyzers/bookends"
	"lesiw.io/tools/analyzers/linelen"
)

// A command sets the width it runs with; the library default is no limit.
func init() { linelen.Config.Len = 79 }
func main() { singlechecker.Main(bookends.Analyzer) }
