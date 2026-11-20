// Command errorsastypeshadow reports an errors.AsType call that tests a zero
// value from an earlier call in the same if/else chain.
package main

import (
	"lesiw.io/tools/analyzers/errorsastypeshadow"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(errorsastypeshadow.Analyzer) }
