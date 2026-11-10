// Command typeunparen reports redundant parentheses in a type expression.
package main

import (
	"lesiw.io/tools/analyzers/typeunparen"
	"lesiw.io/tools/singlechecker"
)

func main() { singlechecker.Main(typeunparen.Analyzer) }
