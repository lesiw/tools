// Command resetvalue reports a Reset result that decides whether to drain a
// timer.
package main

import (
	"lesiw.io/tools/analyzers/resetvalue"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(resetvalue.Analyzer) }
