// Command ctxguard reports redundant ctx.Err() pre-checks.
package main

import (
	"lesiw.io/tools/analyzers/ctxguard"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(ctxguard.Analyzer) }
