// Command typeassertchain reports a chain of type assertions on one value.
package main

import (
	"lesiw.io/tools/analyzers/typeassertchain"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(typeassertchain.Analyzer) }
