// Command preferdecoderune reports a string converted to a rune slice to read
// its first rune.
package main

import (
	"lesiw.io/tools/analyzers/preferdecoderune"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(preferdecoderune.Analyzer) }
