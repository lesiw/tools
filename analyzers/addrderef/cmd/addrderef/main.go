// Command addrderef reports the address of a pointer indirection.
package main

import (
	"lesiw.io/tools/analyzers/addrderef"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(addrderef.Analyzer) }
