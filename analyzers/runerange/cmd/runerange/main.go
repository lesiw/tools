// Command runerange reports a range over a string converted to a rune slice.
package main

import (
	"lesiw.io/tools/analyzers/runerange"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(runerange.Analyzer) }
