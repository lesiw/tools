// Command sloppyreassign reports an assignment whose variable is not used
// outside its statement.
package main

import (
	"lesiw.io/tools/analyzers/linelen"
	"lesiw.io/tools/analyzers/sloppyreassign"
	"lesiw.io/tools/multichecker"
)

// A command sets the width it runs with; the library default is no limit.
func init() { linelen.Config.Len = 79 }
func main() { multichecker.Main(sloppyreassign.Analyzer) }
