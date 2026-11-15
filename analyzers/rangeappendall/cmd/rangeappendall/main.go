// Command rangeappendall reports a range loop that appends each element of a
// slice one at a time.
package main

import (
	"lesiw.io/tools/analyzers/rangeappendall"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(rangeappendall.Analyzer) }
