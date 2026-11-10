// Command selfassert reports a type assertion from a type to itself.
package main

import (
	"lesiw.io/tools/analyzers/selfassert"
	"lesiw.io/tools/singlechecker"
)

func main() { singlechecker.Main(selfassert.Analyzer) }
