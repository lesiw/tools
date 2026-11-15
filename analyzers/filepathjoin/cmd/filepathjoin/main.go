// Command filepathjoin reports a path concatenated with a literal separator.
package main

import (
	"lesiw.io/tools/analyzers/filepathjoin"
	"lesiw.io/tools/analyzers/linelen"
	"lesiw.io/tools/multichecker"
)

func init() { linelen.Config.Len = 79 }
func main() { multichecker.Main(filepathjoin.Analyzer) }
