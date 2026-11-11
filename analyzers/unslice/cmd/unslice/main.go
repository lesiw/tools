// Command unslice reports a slice expression that takes the whole value.
package main

import (
	"lesiw.io/tools/analyzers/unslice"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(unslice.Analyzer) }
