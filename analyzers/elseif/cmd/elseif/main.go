// Command elseif reports an if nested in an else block.
package main

import (
	"lesiw.io/tools/analyzers/elseif"
	"lesiw.io/tools/singlechecker"
)

func main() { singlechecker.Main(elseif.Analyzer) }
