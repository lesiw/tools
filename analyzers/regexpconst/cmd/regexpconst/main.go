// Command regexpconst reports a constant regexp pattern that does not compile.
package main

import (
	"lesiw.io/tools/analyzers/regexpconst"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(regexpconst.Analyzer) }
