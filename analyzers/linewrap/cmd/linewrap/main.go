// Command linewrap reports statement headers that wrap.
package main

import (
	"lesiw.io/tools/analyzers/linewrap"
	"lesiw.io/tools/singlechecker"
)

func main() { singlechecker.Main(linewrap.Analyzer) }
