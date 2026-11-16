// Command nilerrwrap reports a fmt.Errorf %w verb with no error to wrap.
package main

import (
	"lesiw.io/tools/analyzers/nilerrwrap"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(nilerrwrap.Analyzer) }
