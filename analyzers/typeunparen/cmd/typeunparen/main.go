// Command typeunparen reports redundant parentheses in a type expression.
package main

import (
	"lesiw.io/tools/analyzers/typeunparen"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(typeunparen.Analyzer) }
