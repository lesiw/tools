// Command nilchecklen reports a nil check that a len comparison makes
// redundant.
package main

import (
	"lesiw.io/tools/analyzers/nilchecklen"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(nilchecklen.Analyzer) }
