// Command fieldcopy reports a struct literal that copies every field of a
// same-shaped struct whose tags differ.
package main

import (
	"lesiw.io/tools/analyzers/fieldcopy"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(fieldcopy.Analyzer) }
