// Command sprintfquotedstring reports a format string that quotes a %s by
// hand.
package main

import (
	"lesiw.io/tools/analyzers/sprintfquotedstring"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(sprintfquotedstring.Analyzer) }
