// Command unusedfunc reports a package-level declaration nothing in the
// package refers to.
package main

import (
	"lesiw.io/tools/analyzers/unusedfunc"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(unusedfunc.Analyzer) }
