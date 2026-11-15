// Command mapkey reports a map key that is unhashable.
package main

import (
	"lesiw.io/tools/analyzers/mapkey"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(mapkey.Analyzer) }
