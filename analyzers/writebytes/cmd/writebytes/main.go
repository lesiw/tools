// Command writebytes reports a byte slice converted to a string for
// io.WriteString.
package main

import (
	"lesiw.io/tools/analyzers/writebytes"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(writebytes.Analyzer) }
