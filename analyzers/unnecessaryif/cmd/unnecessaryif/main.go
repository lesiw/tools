// Command unnecessaryif reports an if whose else block contains a single if.
package main

import (
	"lesiw.io/tools/analyzers/unnecessaryif"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(unnecessaryif.Analyzer) }
