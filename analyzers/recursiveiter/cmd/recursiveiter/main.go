// Command recursiveiter reports an iterator that ranges over a call to itself.
package main

import (
	"lesiw.io/tools/analyzers/recursiveiter"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(recursiveiter.Analyzer) }
