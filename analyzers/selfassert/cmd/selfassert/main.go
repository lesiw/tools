// Command selfassert reports a type assertion from a type to itself.
package main

import (
	"lesiw.io/tools/analyzers/selfassert"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(selfassert.Analyzer) }
