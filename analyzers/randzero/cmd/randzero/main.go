// Command randzero reports a call to math/rand or math/rand/v2 whose upper
// bound is one.
package main

import (
	"lesiw.io/tools/analyzers/randzero"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(randzero.Analyzer) }
