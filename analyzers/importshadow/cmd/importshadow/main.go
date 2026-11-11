// Command importshadow reports a declaration that shadows an imported package
// name.
package main

import (
	"lesiw.io/tools/analyzers/importshadow"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(importshadow.Analyzer) }
