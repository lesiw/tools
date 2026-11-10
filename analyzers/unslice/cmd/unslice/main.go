// Command unslice reports a slice expression that takes the whole value.
package main

import (
	"lesiw.io/tools/analyzers/unslice"
	"lesiw.io/tools/singlechecker"
)

func main() { singlechecker.Main(unslice.Analyzer) }
