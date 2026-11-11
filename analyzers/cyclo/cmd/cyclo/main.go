// Command cyclo reports functions whose cyclomatic complexity exceeds the
// limit.
package main

import (
	"lesiw.io/tools/analyzers/cyclo"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(cyclo.Analyzer) }
