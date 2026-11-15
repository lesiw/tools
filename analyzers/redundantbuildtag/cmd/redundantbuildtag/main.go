// Command redundantbuildtag reports a build constraint the file name implies.
package main

import (
	"lesiw.io/tools/analyzers/redundantbuildtag"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(redundantbuildtag.Analyzer) }
