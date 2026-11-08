// Command strictvar reports variable declarations that deviate from strict
// form.
package main

import (
	"golang.org/x/tools/go/analysis/singlechecker"

	"lesiw.io/tools/analyzers/strictvar"
)

func main() { singlechecker.Main(strictvar.Analyzer) }
