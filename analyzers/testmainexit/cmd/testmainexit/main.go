// Command testmainexit reports a TestMain that runs the tests and returns.
package main

import (
	"lesiw.io/tools/analyzers/testmainexit"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(testmainexit.Analyzer) }
