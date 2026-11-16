// Command rangestring reports a range over a string that only counts runes.
package main

import (
	"lesiw.io/tools/analyzers/rangestring"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(rangestring.Analyzer) }
