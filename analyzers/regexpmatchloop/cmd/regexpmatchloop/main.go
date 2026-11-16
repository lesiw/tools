// Command regexpmatchloop reports a regexp match function called in a loop
// with a constant pattern.
package main

import (
	"lesiw.io/tools/analyzers/regexpmatchloop"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(regexpmatchloop.Analyzer) }
