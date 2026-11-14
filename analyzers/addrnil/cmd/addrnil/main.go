// Command addrnil reports a comparison of an address with nil that is always
// the same.
package main

import (
	"lesiw.io/tools/analyzers/addrnil"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(addrnil.Analyzer) }
