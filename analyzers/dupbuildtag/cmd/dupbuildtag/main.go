// Command dupbuildtag reports a // +build line whose condition an earlier
// line already gives.
package main

import (
	"lesiw.io/tools/analyzers/dupbuildtag"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(dupbuildtag.Analyzer) }
