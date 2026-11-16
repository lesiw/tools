// Command textblock reports a text block built a line at a time from string
// constants.
package main

import (
	"lesiw.io/tools/analyzers/textblock"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(textblock.Analyzer) }
