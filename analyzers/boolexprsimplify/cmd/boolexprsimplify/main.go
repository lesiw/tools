// Command boolexprsimplify reports a boolean expression with a redundant
// negation or comparison.
package main

import (
	"lesiw.io/tools/analyzers/boolexprsimplify"
	"lesiw.io/tools/analyzers/linelen"
	"lesiw.io/tools/multichecker"
)

func init() { linelen.Config.Len = 79 }

func main() { multichecker.Main(boolexprsimplify.Analyzer) }
