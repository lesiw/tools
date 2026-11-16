// Command dupoption reports an option that a later argument repeats.
package main

import (
	"lesiw.io/tools/analyzers/dupoption"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(dupoption.Analyzer) }
