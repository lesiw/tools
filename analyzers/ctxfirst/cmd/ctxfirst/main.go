// Command ctxfirst reports a context parameter that is not the first one.
package main

import (
	"lesiw.io/tools/analyzers/ctxfirst"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(ctxfirst.Analyzer) }
