// Command unlabelstmt reports a redundant label.
package main

import (
	"lesiw.io/tools/analyzers/unlabelstmt"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(unlabelstmt.Analyzer) }
