// Command badsorting reports a sort.Slice call whose less function does not
// order the elements by the indexes it was given.
package main

import (
	"lesiw.io/tools/analyzers/badsorting"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(badsorting.Analyzer) }
