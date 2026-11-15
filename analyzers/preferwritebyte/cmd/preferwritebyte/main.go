// Command preferwritebyte reports a one-byte slice or string written to a
// writer.
package main

import (
	"lesiw.io/tools/analyzers/preferwritebyte"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(preferwritebyte.Analyzer) }
