// Command commentedoutcode reports a block of commented-out code.
package main

import (
	"lesiw.io/tools/analyzers/commentedoutcode"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(commentedoutcode.Analyzer) }
