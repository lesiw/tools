// Command recvtype reports a type whose methods mix pointer and value
// receivers.
package main

import (
	"lesiw.io/tools/analyzers/recvtype"
	"lesiw.io/tools/singlechecker"
)

func main() { singlechecker.Main(recvtype.Analyzer) }
