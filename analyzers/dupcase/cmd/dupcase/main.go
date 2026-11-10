// Command dupcase reports a case that names an expression an earlier case
// named.
package main

import (
	"lesiw.io/tools/analyzers/dupcase"
	"lesiw.io/tools/singlechecker"
)

func main() { singlechecker.Main(dupcase.Analyzer) }
