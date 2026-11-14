// Command methodexprcall reports a method expression called with its
// receiver.
package main

import (
	"lesiw.io/tools/analyzers/methodexprcall"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(methodexprcall.Analyzer) }
