// Command unwrap reports wrapped constructs that fit on one line.
package main

import (
	"lesiw.io/tools/analyzers/linelen"
	"lesiw.io/tools/analyzers/unwrap"
	"lesiw.io/tools/singlechecker"
)

// A command sets the width it runs with; the library default is no limit.
func init() { linelen.Config.Len = 79 }
func main() { singlechecker.Main(unwrap.Analyzer) }
