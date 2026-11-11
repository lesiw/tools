// Command unusedrecv reports a receiver that the method body never reads.
package main

import (
	"lesiw.io/tools/analyzers/unusedrecv"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(unusedrecv.Analyzer) }
