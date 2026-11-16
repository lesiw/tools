// Command errreturn reports an if that returns the value it tests against
// nil, followed by a return of nil.
package main

import (
	"lesiw.io/tools/analyzers/errreturn"
	"lesiw.io/tools/analyzers/linelen"
	"lesiw.io/tools/multichecker"
)

func init() { linelen.Config.Len = 79 }
func main() { multichecker.Main(errreturn.Analyzer) }
