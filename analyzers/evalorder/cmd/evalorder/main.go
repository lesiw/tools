// Command evalorder reports a return that reads a value and passes its address
// to a call.
package main

import (
	"lesiw.io/tools/analyzers/evalorder"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(evalorder.Analyzer) }
