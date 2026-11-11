// Command unlambda reports a function literal that only forwards its
// arguments.
package main

import (
	"lesiw.io/tools/analyzers/unlambda"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(unlambda.Analyzer) }
