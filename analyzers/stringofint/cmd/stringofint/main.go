// Command stringofint reports a conversion of an integer to a string.
package main

import (
	"lesiw.io/tools/analyzers/stringofint"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(stringofint.Analyzer) }
