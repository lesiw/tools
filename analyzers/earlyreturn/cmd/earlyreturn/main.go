// Command earlyreturn reports a return that chains conditions past the width.
package main

import (
	"lesiw.io/tools/analyzers/earlyreturn"
	"lesiw.io/tools/analyzers/linelen"
	"lesiw.io/tools/multichecker"
)

func init() { linelen.Config.Len = 79 }
func main() { multichecker.Main(earlyreturn.Analyzer) }
