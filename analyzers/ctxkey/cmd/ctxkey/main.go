// Command ctxkey reports a context.WithValue key of a built-in type.
package main

import (
	"lesiw.io/tools/analyzers/ctxkey"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(ctxkey.Analyzer) }
