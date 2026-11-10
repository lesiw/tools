// Command emptystring reports a string compared with its own length.
package main

import (
	"lesiw.io/tools/analyzers/emptystring"
	"lesiw.io/tools/singlechecker"
)

func main() { singlechecker.Main(emptystring.Analyzer) }
