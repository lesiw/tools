// Command nancompare reports a comparison with math.NaN().
package main

import (
	"lesiw.io/tools/analyzers/linelen"
	"lesiw.io/tools/analyzers/nancompare"
	"lesiw.io/tools/multichecker"
)

// A command sets the width it runs with; the library default is no limit.
func init() { linelen.Config.Len = 79 }
func main() { multichecker.Main(nancompare.Analyzer) }
