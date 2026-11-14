// Command unnecessaryblock reports a redundant block statement.
package main

import (
	"lesiw.io/tools/analyzers/unnecessaryblock"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(unnecessaryblock.Analyzer) }
