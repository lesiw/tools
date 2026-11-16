// Command ctxvaluetype reports a context.WithValue call whose value argument
// is the nil literal.
package main

import (
	"lesiw.io/tools/analyzers/ctxvaluetype"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(ctxvaluetype.Analyzer) }
