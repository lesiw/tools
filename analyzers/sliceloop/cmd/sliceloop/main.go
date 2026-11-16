// Command sliceloop reports a loop that copies one slice into another.
package main

import (
	"lesiw.io/tools/analyzers/sliceloop"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(sliceloop.Analyzer) }
