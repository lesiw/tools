// Command dupelseif reports an else-if whose condition repeats an earlier one
// in the chain.
package main

import (
	"lesiw.io/tools/analyzers/dupelseif"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(dupelseif.Analyzer) }
