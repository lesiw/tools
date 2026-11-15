// Command rangevaladdress reports an address taken from a range loop's
// variable and kept past the iteration.
package main

import (
	"lesiw.io/tools/analyzers/rangevaladdress"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(rangevaladdress.Analyzer) }
