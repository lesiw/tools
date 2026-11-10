// Command initialisms reports an initialism spelled in mixed case.
package main

import (
	"lesiw.io/tools/analyzers/initialisms"
	"lesiw.io/tools/singlechecker"
)

func main() { singlechecker.Main(initialisms.Analyzer) }
