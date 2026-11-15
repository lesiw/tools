// Command typedefirst reports a type declared after the methods on it.
package main

import (
	"lesiw.io/tools/analyzers/typedefirst"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(typedefirst.Analyzer) }
