// Command dotimport reports a dot import in a file that is not a test.
package main

import (
	"lesiw.io/tools/analyzers/dotimport"
	"lesiw.io/tools/analyzers/linelen"
	"lesiw.io/tools/multichecker"
)

// A command sets the width it runs with; the library default is no limit.
func init() { linelen.Config.Len = 79 }
func main() { multichecker.Main(dotimport.Analyzer) }
