// Command unreachablecase reports a type in a type switch case that an earlier
// interface case matches.
package main

import (
	"lesiw.io/tools/analyzers/unreachablecase"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(unreachablecase.Analyzer) }
