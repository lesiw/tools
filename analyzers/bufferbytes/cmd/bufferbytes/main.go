// Command bufferbytes reports a bytes.Buffer converted to a string or a byte
// slice.
package main

import (
	"lesiw.io/tools/analyzers/bufferbytes"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(bufferbytes.Analyzer) }
