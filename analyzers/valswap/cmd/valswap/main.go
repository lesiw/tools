// Command valswap reports a swap written through a temporary.
package main

import (
	"lesiw.io/tools/analyzers/valswap"
	"lesiw.io/tools/singlechecker"
)

func main() { singlechecker.Main(valswap.Analyzer) }
