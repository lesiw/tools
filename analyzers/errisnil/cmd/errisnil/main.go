// Command errisnil reports an error compared with nil twice in one boolean
// expression.
package main

import (
	"lesiw.io/tools/analyzers/errisnil"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(errisnil.Analyzer) }
