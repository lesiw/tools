// Command indexalloc reports a byte slice searched as a string.
package main

import (
	"lesiw.io/tools/analyzers/indexalloc"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(indexalloc.Analyzer) }
