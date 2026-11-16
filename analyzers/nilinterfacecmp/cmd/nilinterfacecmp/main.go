// Command nilinterfacecmp reports a comparison of an interface holding a typed
// nil with nil.
package main

import (
	"lesiw.io/tools/analyzers/nilinterfacecmp"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(nilinterfacecmp.Analyzer) }
