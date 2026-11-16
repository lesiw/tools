// Command timeuntil reports time.Now().Add(d) compared with a time.Time.
package main

import (
	"lesiw.io/tools/analyzers/timeuntil"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(timeuntil.Analyzer) }
