// Command lowerfold reports a comparison of two strings converted to lower or
// upper case.
package main

import (
	"lesiw.io/tools/analyzers/lowerfold"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(lowerfold.Analyzer) }
