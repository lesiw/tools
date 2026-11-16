// Command encodeoverlap reports an encoder called with a destination and a
// source at the same index of one backing array.
package main

import (
	"lesiw.io/tools/analyzers/encodeoverlap"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(encodeoverlap.Analyzer) }
