// Command docsyntax reports markup syntax in Go comments.
package main

import (
	"lesiw.io/tools/analyzers/docsyntax"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(docsyntax.Analyzer) }
