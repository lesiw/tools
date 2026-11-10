// Command unlambda reports a function literal that only forwards its
// arguments.
package main

import (
	"lesiw.io/tools/analyzers/unlambda"
	"lesiw.io/tools/singlechecker"
)

func main() { singlechecker.Main(unlambda.Analyzer) }
