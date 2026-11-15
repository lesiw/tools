// Command unsignedcmp reports a comparison of an unsigned value against a
// limit of its type.
package main

import (
	"lesiw.io/tools/analyzers/unsignedcmp"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(unsignedcmp.Analyzer) }
