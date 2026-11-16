// Command makecap reports a make capacity argument equal to the default.
package main

import (
	"lesiw.io/tools/analyzers/makecap"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(makecap.Analyzer) }
