// Command duplicatedcond reports an if whose condition repeats the condition
// of the if it is nested in.
package main

import (
	"lesiw.io/tools/analyzers/duplicatedcond"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(duplicatedcond.Analyzer) }
