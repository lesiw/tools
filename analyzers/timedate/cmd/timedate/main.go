// Command timedate reports a numeric date written in a format that does not
// sort.
package main

import (
	"lesiw.io/tools/analyzers/timedate"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(timedate.Analyzer) }
