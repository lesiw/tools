// Command docsyntax reports markup syntax in Go comments.
package main

import (
	"lesiw.io/tools/analyzers/docsyntax"
	"lesiw.io/tools/singlechecker"
)

func main() { singlechecker.Main(docsyntax.Analyzer) }
