// Command deprecated reports a use of a deprecated identifier or package.
package main

import (
	"lesiw.io/tools/analyzers/deprecated"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(deprecated.Analyzer) }
