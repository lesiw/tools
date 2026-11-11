// Command dupimport reports one package imported twice under different names.
package main

import (
	"lesiw.io/tools/analyzers/dupimport"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(dupimport.Analyzer) }
