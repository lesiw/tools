// Command preferfprint reports a formatted string written to a writer.
package main

import (
	"lesiw.io/tools/analyzers/preferfprint"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(preferfprint.Analyzer) }
