// Command modone reports a binary remainder operation with a divisor of one.
package main

import (
	"lesiw.io/tools/analyzers/modone"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(modone.Analyzer) }
