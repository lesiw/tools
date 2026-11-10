// Command deprecated reports a deprecation that godoc will not read.
package main

import (
	"lesiw.io/tools/analyzers/deprecated"
	"lesiw.io/tools/singlechecker"
)

func main() { singlechecker.Main(deprecated.Analyzer) }
