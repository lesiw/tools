// Command appendassign reports an append result assigned to another slice the
// function still reads.
package main

import (
	"lesiw.io/tools/analyzers/appendassign"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(appendassign.Analyzer) }
