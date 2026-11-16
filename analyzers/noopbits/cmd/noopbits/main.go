// Command noopbits reports a bitwise operation with a zero literal or iota
// zero.
package main

import (
	"lesiw.io/tools/analyzers/noopbits"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(noopbits.Analyzer) }
