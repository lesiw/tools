// Command modifiesparam reports a dead write to a parameter.
package main

import (
	"lesiw.io/tools/analyzers/modifiesparam"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(modifiesparam.Analyzer) }
