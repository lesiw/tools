// Command defermisuse reports a recover call or a return from the wrong
// function.
package main

import (
	"lesiw.io/tools/analyzers/defermisuse"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(defermisuse.Analyzer) }
