// Command importshadow reports a declaration that shadows an imported package
// name.
package main

import (
	"lesiw.io/tools/analyzers/importshadow"
	"lesiw.io/tools/singlechecker"
)

func main() { singlechecker.Main(importshadow.Analyzer) }
