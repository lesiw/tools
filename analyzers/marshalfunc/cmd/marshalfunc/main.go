// Command marshalfunc reports a json marshal of a value whose type contains a
// channel or a func.
package main

import (
	"lesiw.io/tools/analyzers/marshalfunc"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(marshalfunc.Analyzer) }
