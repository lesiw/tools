// Command predictivetemp reports a file created at a fixed path in a shared
// directory.
package main

import (
	"lesiw.io/tools/analyzers/linelen"
	"lesiw.io/tools/analyzers/predictivetemp"
	"lesiw.io/tools/multichecker"
)

func init() { linelen.Config.Len = 79 }
func main() { multichecker.Main(predictivetemp.Analyzer) }
