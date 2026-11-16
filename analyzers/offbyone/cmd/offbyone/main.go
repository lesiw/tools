// Command offbyone reports an index that is always out of range.
package main

import (
	"lesiw.io/tools/analyzers/offbyone"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(offbyone.Analyzer) }
