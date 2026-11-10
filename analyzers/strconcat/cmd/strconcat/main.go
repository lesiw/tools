// Command strconcat reports a two- or three-element Join with a literal
// separator.
package main

import (
	"lesiw.io/tools/analyzers/strconcat"
	"lesiw.io/tools/singlechecker"
)

func main() { singlechecker.Main(strconcat.Analyzer) }
