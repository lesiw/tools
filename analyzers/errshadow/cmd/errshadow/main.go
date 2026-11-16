// Command errshadow reports a declaration in an if body that shadows the
// tested error.
package main

import (
	"lesiw.io/tools/analyzers/errshadow"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(errshadow.Analyzer) }
