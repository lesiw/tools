// Command nilmapassign reports an assignment to an element of a nil map.
package main

import (
	"lesiw.io/tools/analyzers/nilmapassign"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(nilmapassign.Analyzer) }
