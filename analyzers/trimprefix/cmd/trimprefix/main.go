// Command trimprefix reports a HasPrefix check around a TrimPrefix of the
// same prefix.
package main

import (
	"lesiw.io/tools/analyzers/trimprefix"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(trimprefix.Analyzer) }
