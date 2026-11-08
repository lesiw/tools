// Command tidytypes reports redundant type declarations in parameters and
// results.
package main

import (
	"golang.org/x/tools/go/analysis/singlechecker"

	"lesiw.io/tools/analyzers/tidytypes"
)

func main() { singlechecker.Main(tidytypes.Analyzer) }
