// Command mathintconv reports a math rounding call on an integer converted to
// a float.
package main

import (
	"lesiw.io/tools/analyzers/mathintconv"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(mathintconv.Analyzer) }
