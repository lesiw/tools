// Command timeafter reports timer ceremony that can be replaced by time.After.
package main

import (
	"golang.org/x/tools/go/analysis/singlechecker"

	"lesiw.io/tools/analyzers/timeafter"
)

func main() { singlechecker.Main(timeafter.Analyzer) }
