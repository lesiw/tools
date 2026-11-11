// Command recvname reports a receiver name that differs from its type's other
// methods.
package main

import (
	"lesiw.io/tools/analyzers/recvname"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(recvname.Analyzer) }
