// Command regexpmust reports a regexp compiled from a constant with its error
// dropped.
package main

import (
	"lesiw.io/tools/analyzers/regexpmust"
	"lesiw.io/tools/singlechecker"
)

func main() { singlechecker.Main(regexpmust.Analyzer) }
