// Command preferstringwriter reports a converted string written to a writer.
package main

import (
	"lesiw.io/tools/analyzers/preferstringwriter"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(preferstringwriter.Analyzer) }
