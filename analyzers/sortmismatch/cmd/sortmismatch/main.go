// Command sortmismatch reports a sort.Slice call whose less function does not
// name the slice.
package main

import (
	"lesiw.io/tools/analyzers/sortmismatch"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(sortmismatch.Analyzer) }
