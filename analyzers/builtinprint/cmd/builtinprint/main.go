// Command builtinprint reports calls to the predeclared print and println
// functions.
package main

import (
	"lesiw.io/tools/analyzers/builtinprint"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(builtinprint.Analyzer) }
