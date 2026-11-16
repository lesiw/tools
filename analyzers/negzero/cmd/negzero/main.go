// Command negzero reports a unary minus on a floating-point or complex
// constant zero.
package main

import (
	"lesiw.io/tools/analyzers/negzero"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(negzero.Analyzer) }
