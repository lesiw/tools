// Command timeequal2 reports a time.Time compared with the zero-value literal.
package main

import (
	"lesiw.io/tools/analyzers/linelen"
	"lesiw.io/tools/analyzers/timeequal2"
	"lesiw.io/tools/multichecker"
)

func init() { linelen.Config.Len = 79 }
func main() { multichecker.Main(timeequal2.Analyzer) }
