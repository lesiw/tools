// Command errorlast reports a signature whose error result is not the last
// one.
package main

import (
	"lesiw.io/tools/analyzers/errorlast"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(errorlast.Analyzer) }
