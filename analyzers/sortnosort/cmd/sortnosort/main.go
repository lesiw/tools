// Command sortnosort reports an assignment that converts a slice to a sort
// slice type and assigns it back.
package main

import (
	"lesiw.io/tools/analyzers/sortnosort"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(sortnosort.Analyzer) }
