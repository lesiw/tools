// Command sortstrings reports sort.Strings, sort.Ints, sort.Float64s, and
// sort.Slice over a slice of an ordered type that cannot hold a NaN.
package main

import (
	"lesiw.io/tools/analyzers/sortstrings"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(sortstrings.Analyzer) }
