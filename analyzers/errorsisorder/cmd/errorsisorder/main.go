// Command errorsisorder reports a call to errors.Is whose first argument is a
// package-level error variable from another package.
package main

import (
	"lesiw.io/tools/analyzers/errorsisorder"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(errorsisorder.Analyzer) }
