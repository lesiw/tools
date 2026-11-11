// Command emptydecl reports an empty declaration block.
package main

import (
	"lesiw.io/tools/analyzers/emptydecl"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(emptydecl.Analyzer) }
