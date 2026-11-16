// Command maplenloop reports a call to len on the map a range statement
// iterates.
package main

import (
	"lesiw.io/tools/analyzers/linelen"
	"lesiw.io/tools/analyzers/maplenloop"
	"lesiw.io/tools/multichecker"
)

func init() { linelen.Config.Len = 79 }

func main() { multichecker.Main(maplenloop.Analyzer) }
