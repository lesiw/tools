// Command bookends reports bracket layout that does not match the shape of the
// list.
package main

import (
	"lesiw.io/tools/analyzers/bookends"
	"lesiw.io/tools/analyzers/linelen"
	"lesiw.io/tools/multichecker"
)

func init() { linelen.Config.Len = 79 }
func main() { multichecker.Main(bookends.Analyzer) }
