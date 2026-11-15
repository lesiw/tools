// Command deepexit reports a program exit outside main or init.
package main

import (
	"lesiw.io/tools/analyzers/deepexit"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(deepexit.Analyzer) }
