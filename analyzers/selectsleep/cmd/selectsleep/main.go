// Command selectsleep reports a select statement with a single case that
// receives from time.After and discards the value.
package main

import (
	"lesiw.io/tools/analyzers/selectsleep"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(selectsleep.Analyzer) }
