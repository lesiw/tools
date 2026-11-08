// Command tidytypes reports redundant type declarations in parameters and
// results.
package main

import (
	"lesiw.io/tools/analyzers/tidytypes"
	"lesiw.io/tools/singlechecker"
)

func main() { singlechecker.Main(tidytypes.Analyzer) }
