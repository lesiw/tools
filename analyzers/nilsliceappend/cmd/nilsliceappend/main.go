// Command nilsliceappend reports an append to a nil slice where a slice
// literal gives the result.
package main

import (
	"lesiw.io/tools/analyzers/nilsliceappend"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(nilsliceappend.Analyzer) }
