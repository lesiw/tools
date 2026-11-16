// Command breakinselect reports a break at the end of a select case in a loop
// when it leaves only the select.
package main

import (
	"lesiw.io/tools/analyzers/breakinselect"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(breakinselect.Analyzer) }
