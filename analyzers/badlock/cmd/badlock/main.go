// Command badlock reports a deferred call that does not release the lock taken
// in the statement before it.
package main

import (
	"lesiw.io/tools/analyzers/badlock"
	"lesiw.io/tools/analyzers/linelen"
	"lesiw.io/tools/multichecker"
)

func init() { linelen.Config.Len = 79 }
func main() { multichecker.Main(badlock.Analyzer) }
