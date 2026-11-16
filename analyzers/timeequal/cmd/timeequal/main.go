// Command timeequal reports a time.Time compared with == or !=, except the
// zero literal.
package main

import (
	"lesiw.io/tools/analyzers/linelen"
	"lesiw.io/tools/analyzers/timeequal"
	"lesiw.io/tools/multichecker"
)

func init() { linelen.Config.Len = 79 }
func main() { multichecker.Main(timeequal.Analyzer) }
