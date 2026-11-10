// Command emptydecl reports an empty declaration block.
package main

import (
	"lesiw.io/tools/analyzers/emptydecl"
	"lesiw.io/tools/singlechecker"
)

func main() { singlechecker.Main(emptydecl.Analyzer) }
