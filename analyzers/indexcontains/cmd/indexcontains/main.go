// Command indexcontains reports a test of a strings.Index or bytes.Index
// result against -1.
package main

import (
	"lesiw.io/tools/analyzers/indexcontains"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(indexcontains.Analyzer) }
