// Command dupcase reports a case that names an expression an earlier case
// named.
package main

import (
	"lesiw.io/tools/analyzers/dupcase"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(dupcase.Analyzer) }
