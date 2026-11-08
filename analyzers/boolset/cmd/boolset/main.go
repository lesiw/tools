// Command boolset reports map[T]bool used as a set.
package main

import (
	"golang.org/x/tools/go/analysis/singlechecker"

	"lesiw.io/tools/analyzers/boolset"
)

func main() { singlechecker.Main(boolset.Analyzer) }
