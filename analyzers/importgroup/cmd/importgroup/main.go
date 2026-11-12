// Command importgroup reports an import block that is not grouped standard,
// other, local.
package main

import (
	"lesiw.io/tools/analyzers/importgroup"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(importgroup.Analyzer) }
