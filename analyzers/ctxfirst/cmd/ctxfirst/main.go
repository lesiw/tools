// Command ctxfirst reports a context parameter that is not the first one.
package main

import (
	"lesiw.io/tools/analyzers/ctxfirst"
	"lesiw.io/tools/singlechecker"
)

func main() { singlechecker.Main(ctxfirst.Analyzer) }
