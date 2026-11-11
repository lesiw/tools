// Command initialisms reports an initialism spelled in mixed case.
package main

import (
	"lesiw.io/tools/analyzers/initialisms"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(initialisms.Analyzer) }
