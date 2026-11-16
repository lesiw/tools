// Command nilfuncall reports a call of a function value that the same block
// set to nil.
package main

import (
	"lesiw.io/tools/analyzers/nilfuncall"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(nilfuncall.Analyzer) }
