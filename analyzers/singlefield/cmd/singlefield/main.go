// Command singlefield reports single-field structs replaceable by their
// underlying type.
package main

import (
	"lesiw.io/tools/analyzers/singlefield"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(singlefield.Analyzer) }
