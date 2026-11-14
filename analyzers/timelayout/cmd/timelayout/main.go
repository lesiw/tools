// Command timelayout reports a time layout that does not spell the reference
// time.
package main

import (
	"lesiw.io/tools/analyzers/timelayout"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(timelayout.Analyzer) }
