// Command looponce reports a loop that runs at most once.
package main

import (
	"lesiw.io/tools/analyzers/looponce"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(looponce.Analyzer) }
