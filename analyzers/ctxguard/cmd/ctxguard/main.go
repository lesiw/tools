// Command ctxguard reports redundant ctx.Err() pre-checks.
package main

import (
	"golang.org/x/tools/go/analysis/singlechecker"

	"lesiw.io/tools/analyzers/ctxguard"
)

func main() { singlechecker.Main(ctxguard.Analyzer) }
