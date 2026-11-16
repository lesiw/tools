// Command deferinloop reports a deferred call in a loop body.
package main

import (
	"lesiw.io/tools/analyzers/deferinloop"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(deferinloop.Analyzer) }
