// Command duplarg reports a call that passes one value where it expects two.
package main

import (
	"lesiw.io/tools/analyzers/duplarg"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(duplarg.Analyzer) }
