// Command ipliteral reports IP addresses written into source as literals.
package main

import (
	"lesiw.io/tools/analyzers/ipliteral"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(ipliteral.Analyzer) }
