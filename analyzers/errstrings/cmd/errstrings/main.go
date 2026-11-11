// Command errstrings reports a capitalized error string or one ending in
// punctuation.
package main

import (
	"lesiw.io/tools/analyzers/errstrings"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(errstrings.Analyzer) }
