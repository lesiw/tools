// Command linelen reports lines longer than the specified character limit.
package main

import (
	"lesiw.io/tools/analyzers/linelen"
	"lesiw.io/tools/singlechecker"
)

func main() { singlechecker.Main(linelen.Analyzer) }
func init() { linelen.Config.Len = 79 }
