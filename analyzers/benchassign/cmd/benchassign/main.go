// Command benchassign reports an assignment to the N field of a testing.B.
package main

import (
	"lesiw.io/tools/analyzers/benchassign"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(benchassign.Analyzer) }
