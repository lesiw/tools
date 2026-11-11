// Command assignop reports an assignment that writes a variable in terms of
// itself.
package main

import (
	"lesiw.io/tools/analyzers/assignop"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(assignop.Analyzer) }
