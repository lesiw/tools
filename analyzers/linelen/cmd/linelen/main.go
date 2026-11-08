// Command linelen reports lines longer than the specified character limit.
package main

import (
	"golang.org/x/tools/go/analysis/singlechecker"

	"lesiw.io/tools/analyzers/linelen"
)

func main() { singlechecker.Main(linelen.Analyzer) }
