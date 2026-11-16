// Command badcall reports a call that a constant or nil argument leaves with
// nothing to do.
package main

import (
	"lesiw.io/tools/analyzers/badcall"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(badcall.Analyzer) }
