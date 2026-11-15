// Command flagname reports a flag name the flag package rejects.
package main

import (
	"lesiw.io/tools/analyzers/flagname"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(flagname.Analyzer) }
