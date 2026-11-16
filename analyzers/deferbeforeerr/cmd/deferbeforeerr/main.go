// Command deferbeforeerr reports a deferred call on a value whose error
// result is unchecked.
package main

import (
	"lesiw.io/tools/analyzers/deferbeforeerr"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(deferbeforeerr.Analyzer) }
