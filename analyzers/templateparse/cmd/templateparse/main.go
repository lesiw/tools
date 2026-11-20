// Command templateparse reports a template that does not parse.
package main

import (
	"lesiw.io/tools/analyzers/templateparse"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(templateparse.Analyzer) }
