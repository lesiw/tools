// Command boolliteral reports a comparison with a boolean literal.
package main

import (
	"lesiw.io/tools/analyzers/boolliteral"
	"lesiw.io/tools/analyzers/linelen"
	"lesiw.io/tools/multichecker"
)

func init() { linelen.Config.Len = 79 }
func main() { multichecker.Main(boolliteral.Analyzer) }
