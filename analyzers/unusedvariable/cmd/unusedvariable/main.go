// Command unusedvariable reports a variable that is declared and never read.
package main

import (
	"lesiw.io/tools/analyzers/unusedvariable"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(unusedvariable.Analyzer) }
