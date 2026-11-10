// Command zerorepeat reports a run of zero bytes built with bytes.Repeat.
package main

import (
	"lesiw.io/tools/analyzers/zerorepeat"
	"lesiw.io/tools/singlechecker"
)

func main() { singlechecker.Main(zerorepeat.Analyzer) }
