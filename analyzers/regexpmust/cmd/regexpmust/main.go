// Command regexpmust reports a regexp compiled from a constant with its error
// dropped.
package main

import (
	"lesiw.io/tools/analyzers/regexpmust"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(regexpmust.Analyzer) }
