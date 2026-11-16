// Command argoverwritten reports a function parameter whose passed value is
// never used.
package main

import (
	"lesiw.io/tools/analyzers/argoverwritten"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(argoverwritten.Analyzer) }
