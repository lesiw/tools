// Command ctxname reports context.Context variables that can be named ctx.
package main

import (
	"lesiw.io/tools/analyzers/ctxname"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(ctxname.Analyzer) }
