// Command unconvert reports a conversion to a type its operand already has.
package main

import (
	"lesiw.io/tools/analyzers/unconvert"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(unconvert.Analyzer) }
