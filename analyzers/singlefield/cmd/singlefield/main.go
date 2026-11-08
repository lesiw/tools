// Command singlefield reports single-field structs replaceable by their
// underlying type.
package main

import (
	"golang.org/x/tools/go/analysis/singlechecker"

	"lesiw.io/tools/analyzers/singlefield"
)

func main() { singlechecker.Main(singlefield.Analyzer) }
