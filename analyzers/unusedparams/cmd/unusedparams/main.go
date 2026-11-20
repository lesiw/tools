// Command unusedparams reports a parameter a declared function never uses.
package main

import (
	"lesiw.io/tools/analyzers/unusedparams"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(unusedparams.Analyzer) }
