// Command testhelpers reports tests bypassing testing.T-native operations.
package main

import (
	"golang.org/x/tools/go/analysis/singlechecker"

	"lesiw.io/tools/analyzers/testhelpers"
)

func main() { singlechecker.Main(testhelpers.Analyzer) }
