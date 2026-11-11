// Command tidytypes reports redundant type declarations in parameters and
// results.
package main

import (
	"lesiw.io/tools/analyzers/tidytypes"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(tidytypes.Analyzer) }
