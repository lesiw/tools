// Command ctxfield reports a context.Context stored in a struct field.
package main

import (
	"lesiw.io/tools/analyzers/ctxfield"
	"lesiw.io/tools/singlechecker"
)

func main() { singlechecker.Main(ctxfield.Analyzer) }
