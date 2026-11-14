// Command redundantalias reports an import alias equal to the declared package
// name and the last element of its path.
package main

import (
	"lesiw.io/tools/analyzers/redundantalias"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(redundantalias.Analyzer) }
