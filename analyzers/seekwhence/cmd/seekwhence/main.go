// Command seekwhence reports a Seek call with an io.Seek* constant in the
// offset argument.
package main

import (
	"lesiw.io/tools/analyzers/seekwhence"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(seekwhence.Analyzer) }
