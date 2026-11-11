// Command uselessconcat reports string literals joined by + where the join
// fits on one line.
package main

import (
	"lesiw.io/tools/analyzers/uselessconcat"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(uselessconcat.Analyzer) }
