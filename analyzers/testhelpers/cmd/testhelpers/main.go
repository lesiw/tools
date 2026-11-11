// Command testhelpers reports tests bypassing testing.T-native operations.
package main

import (
	"lesiw.io/tools/analyzers/testhelpers"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(testhelpers.Analyzer) }
