// Command printbytes reports a byte slice printed with the default format.
package main

import (
	"lesiw.io/tools/analyzers/printbytes"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(printbytes.Analyzer) }
