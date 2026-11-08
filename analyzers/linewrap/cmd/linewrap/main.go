// Command linewrap reports statement headers that wrap.
package main

import (
	"golang.org/x/tools/go/analysis/singlechecker"

	"lesiw.io/tools/analyzers/linewrap"
)

func main() { singlechecker.Main(linewrap.Analyzer) }
