// Command deprecated reports a deprecation that godoc will not read.
package main

import (
	"lesiw.io/tools/analyzers/deprecated"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(deprecated.Analyzer) }
