// Command yoda reports a comparison with the constant on the left.
package main

import (
	"lesiw.io/tools/analyzers/yoda"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(yoda.Analyzer) }
