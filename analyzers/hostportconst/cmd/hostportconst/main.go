// Command hostportconst reports a constant address that a listen function
// cannot parse as host:port.
package main

import (
	"lesiw.io/tools/analyzers/hostportconst"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(hostportconst.Analyzer) }
