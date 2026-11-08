// Command ipliteral reports IP addresses written into source as literals.
package main

import (
	"lesiw.io/tools/analyzers/ipliteral"
	"lesiw.io/tools/singlechecker"
)

func main() { singlechecker.Main(ipliteral.Analyzer) }
