// Command emptystring reports a string compared with its own length.
package main

import (
	"lesiw.io/tools/analyzers/emptystring"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(emptystring.Analyzer) }
