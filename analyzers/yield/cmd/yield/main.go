// Command yield reports a call to a yield function that may be reached after
// an earlier one returned false.
package main

import (
	"lesiw.io/tools/analyzers/yield"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(yield.Analyzer) }
