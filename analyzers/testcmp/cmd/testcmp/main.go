// Command testcmp reports uses of reflect.DeepEqual in tests.
package main

import (
	"golang.org/x/tools/go/analysis/singlechecker"

	"lesiw.io/tools/analyzers/testcmp"
)

func main() { singlechecker.Main(testcmp.Analyzer) }
