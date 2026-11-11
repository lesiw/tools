// Command testcmp reports uses of reflect.DeepEqual in tests.
package main

import (
	"lesiw.io/tools/analyzers/testcmp"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(testcmp.Analyzer) }
