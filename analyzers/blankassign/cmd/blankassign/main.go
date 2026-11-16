// Command blankassign reports an assignment to the blank identifier that has
// no effect.
package main

import (
	"lesiw.io/tools/analyzers/blankassign"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(blankassign.Analyzer) }
