// Command joinseparator reports a path.Join or filepath.Join element ending
// with a redundant separator.
package main

import (
	"lesiw.io/tools/analyzers/joinseparator"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(joinseparator.Analyzer) }
