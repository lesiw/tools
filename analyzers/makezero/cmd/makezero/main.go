// Command makezero reports a slice made with a non-zero length that is
// appended to.
package main

import (
	"lesiw.io/tools/analyzers/makezero"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(makezero.Analyzer) }
