// Command assertresult reports a case that asserts the value its type switch
// already tested.
package main

import (
	"lesiw.io/tools/analyzers/assertresult"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(assertresult.Analyzer) }
