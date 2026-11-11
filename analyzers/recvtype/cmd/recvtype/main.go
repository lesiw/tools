// Command recvtype reports a type whose methods mix pointer and value
// receivers.
package main

import (
	"lesiw.io/tools/analyzers/recvtype"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(recvtype.Analyzer) }
