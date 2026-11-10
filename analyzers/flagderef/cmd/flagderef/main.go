// Command flagderef reports a flag value read before Parse can set it.
package main

import (
	"lesiw.io/tools/analyzers/flagderef"
	"lesiw.io/tools/singlechecker"
)

func main() { singlechecker.Main(flagderef.Analyzer) }
