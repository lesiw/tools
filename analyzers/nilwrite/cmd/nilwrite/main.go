// Command nilwrite reports a Write method that may modify the buffer it was
// given.
package main

import (
	"lesiw.io/tools/analyzers/nilwrite"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(nilwrite.Analyzer) }
