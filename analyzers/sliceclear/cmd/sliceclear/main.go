// Command sliceclear reports a loop that zeroes every element of a slice.
package main

import (
	"lesiw.io/tools/analyzers/sliceclear"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(sliceclear.Analyzer) }
