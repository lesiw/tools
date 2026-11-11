// Command linewrap reports statement headers that wrap.
package main

import (
	"lesiw.io/tools/analyzers/linewrap"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(linewrap.Analyzer) }
