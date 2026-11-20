// Command builtinbound reports a comparison of len or cap with zero that is
// always true or always false.
package main

import (
	"lesiw.io/tools/analyzers/builtinbound"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(builtinbound.Analyzer) }
