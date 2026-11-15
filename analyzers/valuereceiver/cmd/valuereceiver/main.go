// Command valuereceiver reports a dead write to a field of a value receiver.
package main

import (
	"lesiw.io/tools/analyzers/valuereceiver"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(valuereceiver.Analyzer) }
