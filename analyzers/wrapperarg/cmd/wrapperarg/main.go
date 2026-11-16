// Command wrapperarg reports a call that passes an argument a
// convenience wrapper already supplies.
package main

import (
	"lesiw.io/tools/analyzers/wrapperarg"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(wrapperarg.Analyzer) }
