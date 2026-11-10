// Command typeswitchvar reports a type switch whose cases assert the value
// again.
package main

import (
	"lesiw.io/tools/analyzers/typeswitchvar"
	"lesiw.io/tools/singlechecker"
)

func main() { singlechecker.Main(typeswitchvar.Analyzer) }
