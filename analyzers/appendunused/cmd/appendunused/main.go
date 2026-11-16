// Command appendunused reports an append result assigned to the blank
// identifier.
package main

import (
	"lesiw.io/tools/analyzers/appendunused"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(appendunused.Analyzer) }
