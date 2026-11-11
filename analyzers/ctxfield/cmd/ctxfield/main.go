// Command ctxfield reports a context.Context stored in a struct field.
package main

import (
	"lesiw.io/tools/analyzers/ctxfield"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(ctxfield.Analyzer) }
