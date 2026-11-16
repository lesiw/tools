// Command bidicontrol reports a bidirectional control character in a source
// file.
package main

import (
	"lesiw.io/tools/analyzers/bidicontrol"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(bidicontrol.Analyzer) }
