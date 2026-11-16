// Command ifreturn reports an else block after an if that only returns.
package main

import (
	"lesiw.io/tools/analyzers/ifreturn"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(ifreturn.Analyzer) }
