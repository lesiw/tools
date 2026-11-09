// Command cyclo reports functions whose cyclomatic complexity exceeds the
// limit.
package main

import (
	"lesiw.io/tools/analyzers/cyclo"
	"lesiw.io/tools/singlechecker"
)

func main() { singlechecker.Main(cyclo.Analyzer) }
