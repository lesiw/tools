// Command queryadd reports an Add call on the values returned by URL.Query.
package main

import (
	"lesiw.io/tools/analyzers/queryadd"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(queryadd.Analyzer) }
