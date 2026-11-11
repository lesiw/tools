// Command emptyfallthrough reports a case whose whole body is a fallthrough.
package main

import (
	"lesiw.io/tools/analyzers/emptyfallthrough"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(emptyfallthrough.Analyzer) }
