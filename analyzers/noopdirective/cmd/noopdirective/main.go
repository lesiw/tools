// Command noopdirective reports a comment that reads as a //go: directive but
// for whitespace.
package main

import (
	"lesiw.io/tools/analyzers/noopdirective"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(noopdirective.Analyzer) }
