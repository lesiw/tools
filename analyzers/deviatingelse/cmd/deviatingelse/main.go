// Command deviatingelse reports an if/else whose else block does not fall
// through to the statement after the if.
package main

import (
	"lesiw.io/tools/analyzers/deviatingelse"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(deviatingelse.Analyzer) }
