// Command selectempty reports a select statement with no cases.
package main

import (
	"lesiw.io/tools/analyzers/selectempty"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(selectempty.Analyzer) }
