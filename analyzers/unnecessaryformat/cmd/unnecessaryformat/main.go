// Command unnecessaryformat reports a call whose format string has no verb.
package main

import (
	"lesiw.io/tools/analyzers/unnecessaryformat"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(unnecessaryformat.Analyzer) }
