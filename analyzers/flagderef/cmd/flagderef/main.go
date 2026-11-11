// Command flagderef reports a flag value read before Parse can set it.
package main

import (
	"lesiw.io/tools/analyzers/flagderef"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(flagderef.Analyzer) }
