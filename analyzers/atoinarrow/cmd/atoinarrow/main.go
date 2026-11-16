// Command atoinarrow reports a conversion of a strconv.Atoi result to int16,
// or to int32 where int is 64 bits.
package main

import (
	"lesiw.io/tools/analyzers/atoinarrow"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(atoinarrow.Analyzer) }
