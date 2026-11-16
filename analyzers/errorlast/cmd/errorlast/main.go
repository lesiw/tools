// Command errorlast reports a function type whose last error result is
// followed by another result.
package main

import (
	"lesiw.io/tools/analyzers/errorlast"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(errorlast.Analyzer) }
