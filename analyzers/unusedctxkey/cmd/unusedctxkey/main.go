// Command unusedctxkey reports a context key constant that no
// context.WithValue call passes.
package main

import (
	"lesiw.io/tools/analyzers/unusedctxkey"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(unusedctxkey.Analyzer) }
