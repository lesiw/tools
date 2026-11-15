// Command underef reports a redundant dereference in front of a selector.
package main

import (
	"lesiw.io/tools/analyzers/underef"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(underef.Analyzer) }
