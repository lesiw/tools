// Command docstub reports a doc comment that only repeats the declaration's
// name.
package main

import (
	"lesiw.io/tools/analyzers/docstub"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(docstub.Analyzer) }
