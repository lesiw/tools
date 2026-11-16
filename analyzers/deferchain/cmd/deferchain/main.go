// Command deferchain reports a deferred call that returns a function.
package main

import (
	"lesiw.io/tools/analyzers/deferchain"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(deferchain.Analyzer) }
