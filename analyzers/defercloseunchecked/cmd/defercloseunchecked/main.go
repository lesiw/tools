// Command defercloseunchecked reports a deferred Close or Flush that discards
// its error.
package main

import (
	"lesiw.io/tools/analyzers/defercloseunchecked"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(defercloseunchecked.Analyzer) }
