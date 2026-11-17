// Command elseif reports an if nested in an else block.
package main

import (
	"lesiw.io/tools/analyzers/elseif"
	"lesiw.io/tools/analyzers/linelen"
	"lesiw.io/tools/multichecker"
)

func init() { linelen.Config.Len = 79 }
func main() { multichecker.Main(elseif.Analyzer) }
