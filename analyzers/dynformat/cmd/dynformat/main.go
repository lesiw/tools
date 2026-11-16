// Command dynformat reports a printf-style call with a non-constant format
// string and no argument after it.
package main

import (
	"lesiw.io/tools/analyzers/dynformat"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(dynformat.Analyzer) }
