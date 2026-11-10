// Command preferfprint reports a formatted string written to a writer.
package main

import (
	"lesiw.io/tools/analyzers/preferfprint"
	"lesiw.io/tools/singlechecker"
)

func main() { singlechecker.Main(preferfprint.Analyzer) }
