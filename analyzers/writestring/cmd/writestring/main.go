// Command writestring reports a WriteString call whose argument concatenates
// strings.
package main

import (
	"lesiw.io/tools/analyzers/writestring"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(writestring.Analyzer) }
