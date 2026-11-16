// Command assertelse reports a type assertion value read where the assertion
// fails.
package main

import (
	"lesiw.io/tools/analyzers/assertelse"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(assertelse.Analyzer) }
