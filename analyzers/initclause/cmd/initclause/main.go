// Command initclause reports an if or switch init clause that declares
// nothing.
package main

import (
	"lesiw.io/tools/analyzers/initclause"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(initclause.Analyzer) }
