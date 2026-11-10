// Command switchtrue reports a switch whose tag is the literal true.
package main

import (
	"lesiw.io/tools/analyzers/switchtrue"
	"lesiw.io/tools/singlechecker"
)

func main() { singlechecker.Main(switchtrue.Analyzer) }
