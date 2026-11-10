// Command unusedrecv reports a receiver that the method body never reads.
package main

import (
	"lesiw.io/tools/analyzers/unusedrecv"
	"lesiw.io/tools/singlechecker"
)

func main() { singlechecker.Main(unusedrecv.Analyzer) }
