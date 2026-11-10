// Command indexalloc reports a byte slice searched as a string.
package main

import (
	"lesiw.io/tools/analyzers/indexalloc"
	"lesiw.io/tools/singlechecker"
)

func main() { singlechecker.Main(indexalloc.Analyzer) }
