// Command increment reports an increment or decrement written as a compound
// assignment.
package main

import (
	"lesiw.io/tools/analyzers/increment"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(increment.Analyzer) }
