// Command rangelast reports a range loop that keeps only the last element.
package main

import (
	"lesiw.io/tools/analyzers/rangelast"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(rangelast.Analyzer) }
