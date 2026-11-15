// Command badcond reports a condition that is always true or always false.
package main

import (
	"lesiw.io/tools/analyzers/badcond"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(badcond.Analyzer) }
