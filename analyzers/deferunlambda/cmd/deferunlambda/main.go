// Command deferunlambda reports a deferred literal that only calls a function.
package main

import (
	"lesiw.io/tools/analyzers/deferunlambda"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(deferunlambda.Analyzer) }
