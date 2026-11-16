// Command deferinfinite reports a deferred call in a loop that cannot end.
package main

import (
	"lesiw.io/tools/analyzers/deferinfinite"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(deferinfinite.Analyzer) }
