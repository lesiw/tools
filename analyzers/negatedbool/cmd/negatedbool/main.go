// Command negatedbool reports a boolean expression that a negation makes
// constant or redundant.
package main

import (
	"lesiw.io/tools/analyzers/linelen"
	"lesiw.io/tools/analyzers/negatedbool"
	"lesiw.io/tools/multichecker"
)

func init() { linelen.Config.Len = 79 }

func main() { multichecker.Main(negatedbool.Analyzer) }
