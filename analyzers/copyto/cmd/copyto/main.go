// Command copyto reports a loop that copies one slice into another element by
// element.
package main

import (
	"lesiw.io/tools/analyzers/copyto"
	"lesiw.io/tools/analyzers/linelen"
	"lesiw.io/tools/multichecker"
)

func init() { linelen.Config.Len = 79 }
func main() { multichecker.Main(copyto.Analyzer) }
