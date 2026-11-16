// Command dupcutset reports a strings or bytes Trim, TrimLeft, or TrimRight
// call whose literal cutset repeats a character.
package main

import (
	"lesiw.io/tools/analyzers/dupcutset"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(dupcutset.Analyzer) }
