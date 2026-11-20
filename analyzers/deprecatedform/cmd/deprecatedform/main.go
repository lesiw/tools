// Command deprecatedform reports a deprecation that godoc will not read.
package main

import (
	"lesiw.io/tools/analyzers/deprecatedform"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(deprecatedform.Analyzer) }
