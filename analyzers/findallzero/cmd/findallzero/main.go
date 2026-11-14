// Command findallzero reports a FindAll call on a regexp.Regexp with a count
// of zero.
package main

import (
	"lesiw.io/tools/analyzers/findallzero"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(findallzero.Analyzer) }
