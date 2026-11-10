// Command ifelsechain reports an if-else chain that reads as a switch.
package main

import (
	"lesiw.io/tools/analyzers/ifelsechain"
	"lesiw.io/tools/singlechecker"
)

func main() { singlechecker.Main(ifelsechain.Analyzer) }
