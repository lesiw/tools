// Command dotimports reports a dot import.
package main

import (
	"lesiw.io/tools/analyzers/dotimports"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(dotimports.Analyzer) }
