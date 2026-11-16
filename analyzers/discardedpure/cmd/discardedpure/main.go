// Command discardedpure reports a call whose only effect is a discarded
// result.
package main

import (
	"lesiw.io/tools/analyzers/discardedpure"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(discardedpure.Analyzer) }
