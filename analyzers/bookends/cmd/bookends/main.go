// Command bookends reports a closing bracket that is not at the indent of its
// opening bracket.
package main

import (
	"lesiw.io/tools/analyzers/bookends"
	"lesiw.io/tools/analyzers/linelen"
	"lesiw.io/tools/multichecker"
)

// A command sets the width it runs with; the library default is no limit.
func init() { linelen.Config.Len = 79 }
func main() { multichecker.Main(bookends.Analyzer) }
