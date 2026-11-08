// Command testcmp reports uses of reflect.DeepEqual in tests.
package main

import (
	"lesiw.io/tools/analyzers/testcmp"
	"lesiw.io/tools/singlechecker"
)

func main() { singlechecker.Main(testcmp.Analyzer) }
