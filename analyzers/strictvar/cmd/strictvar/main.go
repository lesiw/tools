// Command strictvar reports variable declarations that deviate from strict
// form.
package main

import (
	"lesiw.io/tools/analyzers/strictvar"
	"lesiw.io/tools/singlechecker"
)

func main() { singlechecker.Main(strictvar.Analyzer) }
