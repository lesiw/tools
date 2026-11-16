// Command dubbitshift reports a shift of a fixed-size typed constant by a
// count at least as large as the type's width, or a shift of an unsigned typed
// constant whose result is zero below that width.
package main

import (
	"lesiw.io/tools/analyzers/dubbitshift"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(dubbitshift.Analyzer) }
