// Command uncondrecursion reports a recursive call that runs whenever the
// function runs.
package main

import (
	"lesiw.io/tools/analyzers/uncondrecursion"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(uncondrecursion.Analyzer) }
