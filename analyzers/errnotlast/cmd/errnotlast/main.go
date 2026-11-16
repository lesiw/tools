// Command errnotlast reports a function whose last error result is followed by
// another result.
package main

import (
	"lesiw.io/tools/analyzers/errnotlast"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(errnotlast.Analyzer) }
