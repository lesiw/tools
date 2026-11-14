// Command dupsubexpr reports an operator whose operands are the same value.
package main

import (
	"lesiw.io/tools/analyzers/dupsubexpr"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(dupsubexpr.Analyzer) }
