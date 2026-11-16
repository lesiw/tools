// Command appendself reports an append result assigned to a slice other than
// the one it appends to.
package main

import (
	"lesiw.io/tools/analyzers/appendself"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(appendself.Analyzer) }
