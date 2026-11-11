// Command containscut reports a Contains check whose answer Cut already
// returns.
package main

import (
	"lesiw.io/tools/analyzers/containscut"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(containscut.Analyzer) }
