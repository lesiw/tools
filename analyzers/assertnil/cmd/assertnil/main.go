// Command assertnil reports a redundant nil check on the result of a type
// assertion.
package main

import (
	"lesiw.io/tools/analyzers/assertnil"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(assertnil.Analyzer) }
