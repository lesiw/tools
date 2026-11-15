// Command stringxbytes reports a conversion between string and []byte that
// nothing needs.
package main

import (
	"lesiw.io/tools/analyzers/stringxbytes"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(stringxbytes.Analyzer) }
