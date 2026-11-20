// Command sprintfprint reports a fmt.Sprintf call whose result a print call
// formats again.
package main

import (
	"lesiw.io/tools/analyzers/sprintfprint"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(sprintfprint.Analyzer) }
