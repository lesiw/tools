// Command boolset reports map[T]bool used as a set.
package main

import (
	"lesiw.io/tools/analyzers/boolset"
	"lesiw.io/tools/singlechecker"
)

func main() { singlechecker.Main(boolset.Analyzer) }
