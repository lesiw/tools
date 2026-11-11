// Command strcompare reports a comparison made through strings.Compare or
// bytes.Compare.
package main

import (
	"lesiw.io/tools/analyzers/strcompare"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(strcompare.Analyzer) }
