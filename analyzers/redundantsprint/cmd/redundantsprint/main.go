// Command redundantsprint reports a wrapper that leaves its string operand
// unchanged.
package main

import (
	"lesiw.io/tools/analyzers/redundantsprint"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(redundantsprint.Analyzer) }
