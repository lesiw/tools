// Command defaultcaseorder reports a default clause in the middle of a
// switch, a type switch, or a select.
package main

import (
	"lesiw.io/tools/analyzers/defaultcaseorder"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(defaultcaseorder.Analyzer) }
