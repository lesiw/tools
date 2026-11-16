// Command uselessbreak reports a break at the end of a switch case.
package main

import (
	"lesiw.io/tools/analyzers/uselessbreak"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(uselessbreak.Analyzer) }
