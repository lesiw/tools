// Command unusedparams reports a parameter a function never reads.
package main

import (
	"lesiw.io/tools/analyzers/unusedparams"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(unusedparams.Analyzer) }
