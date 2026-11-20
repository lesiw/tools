// Command simplifyslice reports a slice expression whose high index is the
// length of the slice.
package main

import (
	"lesiw.io/tools/analyzers/simplifyslice"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(simplifyslice.Analyzer) }
