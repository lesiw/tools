// Command testlabel reports a subtest whose name is a string literal.
package main

import (
	"lesiw.io/tools/analyzers/testlabel"
	"lesiw.io/tools/singlechecker"
)

func main() { singlechecker.Main(testlabel.Analyzer) }
