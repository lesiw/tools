// Command deepequalptr reports a reflect.DeepEqual call on a type with an
// Equal method and a pointer field.
package main

import (
	"lesiw.io/tools/analyzers/deepequalptr"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(deepequalptr.Analyzer) }
