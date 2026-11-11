// Command linelen reports lines longer than the specified character limit.
package main

import (
	"lesiw.io/tools/analyzers/linelen"
	"lesiw.io/tools/multichecker"
)

func init() { linelen.Config.Len = 79 }
func main() { multichecker.Main(linelen.Analyzer) }
