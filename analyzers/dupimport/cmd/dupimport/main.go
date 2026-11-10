// Command dupimport reports one package imported twice under different names.
package main

import (
	"lesiw.io/tools/analyzers/dupimport"
	"lesiw.io/tools/singlechecker"
)

func main() { singlechecker.Main(dupimport.Analyzer) }
