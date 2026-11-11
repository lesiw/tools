// Command offby1 reports an index that is always out of range.
package main

import (
	"lesiw.io/tools/analyzers/offby1"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(offby1.Analyzer) }
