// Command simplifyrange reports a range clause that can be written without the
// blank.
package main

import (
	"lesiw.io/tools/analyzers/simplifyrange"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(simplifyrange.Analyzer) }
