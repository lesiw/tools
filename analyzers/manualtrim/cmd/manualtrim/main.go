// Command manualtrim reports an if statement that trims a string prefix by
// reslicing it.
package main

import (
	"lesiw.io/tools/analyzers/manualtrim"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(manualtrim.Analyzer) }
