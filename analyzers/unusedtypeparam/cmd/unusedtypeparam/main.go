// Command unusedtypeparam reports a type parameter a function, method, or type
// alias never uses.
package main

import (
	"lesiw.io/tools/analyzers/unusedtypeparam"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(unusedtypeparam.Analyzer) }
