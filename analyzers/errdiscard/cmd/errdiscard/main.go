// Command errdiscard reports a call whose only result is an error, written as
// a statement.
package main

import (
	"lesiw.io/tools/analyzers/errdiscard"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(errdiscard.Analyzer) }
