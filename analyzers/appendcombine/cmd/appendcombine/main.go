// Command appendcombine reports consecutive appends to the same slice.
package main

import (
	"lesiw.io/tools/analyzers/appendcombine"
	"lesiw.io/tools/singlechecker"
)

func main() { singlechecker.Main(appendcombine.Analyzer) }
