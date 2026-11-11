// Command strictvar reports variable declarations that deviate from strict
// form.
package main

import (
	"lesiw.io/tools/analyzers/strictvar"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(strictvar.Analyzer) }
