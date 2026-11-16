// Command regexpsimplify reports a regexp character class that has a shorter
// spelling.
package main

import (
	"lesiw.io/tools/analyzers/regexpsimplify"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(regexpsimplify.Analyzer) }
