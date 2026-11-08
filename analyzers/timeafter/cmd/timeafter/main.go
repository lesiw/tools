// Command timeafter reports timer ceremony that can be replaced by time.After.
package main

import (
	"lesiw.io/tools/analyzers/timeafter"
	"lesiw.io/tools/singlechecker"
)

func main() { singlechecker.Main(timeafter.Analyzer) }
