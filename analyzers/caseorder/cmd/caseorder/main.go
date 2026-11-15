// Command caseorder reports a switch whose case values are a contiguous run in
// neither ascending nor descending order.
package main

import (
	"lesiw.io/tools/analyzers/caseorder"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(caseorder.Analyzer) }
