// Command selectone reports a select statement with a single case and no
// default.
package main

import (
	"lesiw.io/tools/analyzers/selectone"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(selectone.Analyzer) }
