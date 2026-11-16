// Command builtinshadow reports a declaration that shadows a predeclared
// identifier.
package main

import (
	"lesiw.io/tools/analyzers/builtinshadow"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(builtinshadow.Analyzer) }
