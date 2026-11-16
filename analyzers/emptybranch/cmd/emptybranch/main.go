// Command emptybranch reports an if or else branch with no statements and no
// comment.
package main

import (
	"lesiw.io/tools/analyzers/emptybranch"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(emptybranch.Analyzer) }
