// Command ctxname reports context.Context variables that can be named ctx.
package main

import (
	"golang.org/x/tools/go/analysis/singlechecker"

	"lesiw.io/tools/analyzers/ctxname"
)

func main() { singlechecker.Main(ctxname.Analyzer) }
