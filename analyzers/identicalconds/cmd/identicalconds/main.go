// Command identicalconds reports an else-if that repeats an earlier condition
// in the same chain.
package main

import (
	"lesiw.io/tools/analyzers/identicalconds"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(identicalconds.Analyzer) }
