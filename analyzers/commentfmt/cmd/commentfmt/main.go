// Command commentfmt reports a comment with no space after its slashes.
package main

import (
	"lesiw.io/tools/analyzers/commentfmt"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(commentfmt.Analyzer) }
