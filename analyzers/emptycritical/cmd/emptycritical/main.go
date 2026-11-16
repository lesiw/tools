// Command emptycritical reports a lock taken and released with no statement
// between.
package main

import (
	"lesiw.io/tools/analyzers/emptycritical"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(emptycritical.Analyzer) }
