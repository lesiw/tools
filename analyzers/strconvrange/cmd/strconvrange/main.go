// Command strconvrange reports a strconv argument outside the documented
// values.
package main

import (
	"lesiw.io/tools/analyzers/strconvrange"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(strconvrange.Analyzer) }
