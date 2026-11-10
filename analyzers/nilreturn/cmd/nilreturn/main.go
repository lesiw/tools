// Command nilreturn reports a return of the variable an if just found to be
// nil.
package main

import (
	"lesiw.io/tools/analyzers/nilreturn"
	"lesiw.io/tools/singlechecker"
)

func main() { singlechecker.Main(nilreturn.Analyzer) }
