// Command vargrouptype reports an untyped constant whose default type is not
// the one declared above it.
package main

import (
	"lesiw.io/tools/analyzers/vargrouptype"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(vargrouptype.Analyzer) }
