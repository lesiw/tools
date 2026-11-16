// Command timenowloop reports a time.Now call in a for condition.
package main

import (
	"lesiw.io/tools/analyzers/timenowloop"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(timenowloop.Analyzer) }
