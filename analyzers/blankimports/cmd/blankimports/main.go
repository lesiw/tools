// Command blankimports reports a blank import in a non-main, non-test file
// with no comment naming its path.
package main

import (
	"lesiw.io/tools/analyzers/blankimports"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(blankimports.Analyzer) }
