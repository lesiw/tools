// Command appendloop reports a range loop that appends each element of a slice
// by index.
package main

import (
	"lesiw.io/tools/analyzers/appendloop"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(appendloop.Analyzer) }
