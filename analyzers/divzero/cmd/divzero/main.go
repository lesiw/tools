// Command divzero reports integer division of literals whose quotient is
// zero.
package main

import (
	"lesiw.io/tools/analyzers/divzero"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(divzero.Analyzer) }
