// Command unreachablereturn reports a return statement after a panic or an
// endless loop in the same block.
package main

import (
	"lesiw.io/tools/analyzers/unreachablereturn"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(unreachablereturn.Analyzer) }
