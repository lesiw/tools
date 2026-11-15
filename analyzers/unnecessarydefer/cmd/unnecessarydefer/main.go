// Command unnecessarydefer reports a deferred call that can be replaced by a
// plain call.
package main

import (
	"lesiw.io/tools/analyzers/unnecessarydefer"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(unnecessarydefer.Analyzer) }
