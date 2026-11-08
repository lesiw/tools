// Command unwrap reports wrapped constructs that fit on one line.
package main

import (
	"golang.org/x/tools/go/analysis/singlechecker"

	"lesiw.io/tools/analyzers/linelen"
	"lesiw.io/tools/analyzers/unwrap"
)

// A command sets the width it runs with; the library default is no limit.
func init() { linelen.Config.Len = 79 }
func main() { singlechecker.Main(unwrap.Analyzer) }
