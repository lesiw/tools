// Command testhelpers reports tests bypassing testing.T-native operations.
package main

import (
	"lesiw.io/tools/analyzers/testhelpers"
	"lesiw.io/tools/singlechecker"
)

func main() { singlechecker.Main(testhelpers.Analyzer) }
