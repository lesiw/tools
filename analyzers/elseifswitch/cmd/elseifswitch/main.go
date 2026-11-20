// Command elseifswitch reports an if-else chain that compares one expression
// with ==.
package main

import (
	"lesiw.io/tools/analyzers/elseifswitch"
	"lesiw.io/tools/analyzers/linelen"
	"lesiw.io/tools/multichecker"
)

func init() { linelen.Config.Len = 79 }
func main() { multichecker.Main(elseifswitch.Analyzer) }
