// Command emptylines reports blank lines that separate nothing.
package main

import (
	"lesiw.io/tools/analyzers/emptylines"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(emptylines.Analyzer) }
