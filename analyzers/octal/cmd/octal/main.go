// Command octal reports an octal literal written with a leading zero.
package main

import (
	"lesiw.io/tools/analyzers/octal"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(octal.Analyzer) }
