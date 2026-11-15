// Command wrapperfunc reports a function that only forwards its arguments.
package main

import (
	"lesiw.io/tools/analyzers/wrapperfunc"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(wrapperfunc.Analyzer) }
