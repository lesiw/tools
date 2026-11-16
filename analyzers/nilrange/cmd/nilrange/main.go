// Command nilrange reports a nil check around a range loop over a slice or a
// map.
package main

import (
	"lesiw.io/tools/analyzers/nilrange"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(nilrange.Analyzer) }
