// Command maprange reports a call to maps.Keys or maps.Values in a range.
package main

import (
	"lesiw.io/tools/analyzers/maprange"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(maprange.Analyzer) }
