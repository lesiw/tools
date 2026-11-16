// Command niltimer reports a Timer or Ticker method call on a nil pointer.
package main

import (
	"lesiw.io/tools/analyzers/niltimer"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(niltimer.Analyzer) }
