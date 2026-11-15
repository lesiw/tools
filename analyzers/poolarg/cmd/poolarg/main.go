// Command poolarg reports a value stored in a sync.Pool that is not one
// pointer word.
package main

import (
	"lesiw.io/tools/analyzers/poolarg"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(poolarg.Analyzer) }
