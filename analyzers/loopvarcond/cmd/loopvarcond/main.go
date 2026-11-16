// Command loopvarcond reports a for loop whose condition tests a variable the
// loop never changes.
package main

import (
	"lesiw.io/tools/analyzers/loopvarcond"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(loopvarcond.Analyzer) }
