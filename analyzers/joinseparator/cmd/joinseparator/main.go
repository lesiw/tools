// Command joinseparator reports a join element ending in redundant separators.
package main

import (
	"lesiw.io/tools/analyzers/joinseparator"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(joinseparator.Analyzer) }
