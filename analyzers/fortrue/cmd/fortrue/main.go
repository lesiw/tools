// Command fortrue reports a for statement whose condition is the literal true
// and whose for clause has no init or post statement.
package main

import (
	"lesiw.io/tools/analyzers/fortrue"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(fortrue.Analyzer) }
