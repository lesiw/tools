// Command strconcat reports a two- or three-element Join with a literal
// separator, except a newline text block.
package main

import (
	"lesiw.io/tools/analyzers/strconcat"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(strconcat.Analyzer) }
