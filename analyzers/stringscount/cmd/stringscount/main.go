// Command stringscount reports a loop that counts the elements of a slice with
// a counter.
package main

import (
	"lesiw.io/tools/analyzers/stringscount"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(stringscount.Analyzer) }
