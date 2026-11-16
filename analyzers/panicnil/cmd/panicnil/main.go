// Command panicnil reports a call to panic with the predeclared nil.
package main

import (
	"lesiw.io/tools/analyzers/panicnil"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(panicnil.Analyzer) }
