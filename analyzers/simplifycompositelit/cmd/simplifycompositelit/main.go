// Command simplifycompositelit reports an element or map key that repeats the
// type its composite literal names.
package main

import (
	"lesiw.io/tools/analyzers/simplifycompositelit"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(simplifycompositelit.Analyzer) }
