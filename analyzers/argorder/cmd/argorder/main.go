// Command argorder reports a call whose arguments name the parameters in the
// other order.
package main

import (
	"lesiw.io/tools/analyzers/argorder"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(argorder.Analyzer) }
