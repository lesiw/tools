// Command boolreturn reports two returns of opposite boolean constants under
// an if.
package main

import (
	"lesiw.io/tools/analyzers/boolreturn"
	"lesiw.io/tools/analyzers/linelen"
	"lesiw.io/tools/multichecker"
)

func init() { linelen.Config.Len = 79 }
func main() { multichecker.Main(boolreturn.Analyzer) }
