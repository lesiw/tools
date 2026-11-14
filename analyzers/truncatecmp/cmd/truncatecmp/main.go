// Command truncatecmp reports a comparison through a narrowing integer
// conversion.
package main

import (
	"lesiw.io/tools/analyzers/truncatecmp"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(truncatecmp.Analyzer) }
