// Command deleteguard reports an existence check around a delete of the same
// key.
package main

import (
	"lesiw.io/tools/analyzers/deleteguard"
	"lesiw.io/tools/analyzers/linelen"
	"lesiw.io/tools/multichecker"
)

// A command sets the width it runs with; the library default is no limit.
func init() { linelen.Config.Len = 79 }
func main() { multichecker.Main(deleteguard.Analyzer) }
