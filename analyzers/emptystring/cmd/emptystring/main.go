// Command emptystring reports a test of whether a string is empty, written as
// a comparison of its length with zero.
package main

import (
	"lesiw.io/tools/analyzers/emptystring"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(emptystring.Analyzer) }
