// Command slenstr reports a string converted to a byte slice only to take its
// length.
package main

import (
	"lesiw.io/tools/analyzers/slenstr"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(slenstr.Analyzer) }
