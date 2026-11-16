// Command structtagdup reports a struct tag that names the same key twice.
package main

import (
	"lesiw.io/tools/analyzers/structtagdup"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(structtagdup.Analyzer) }
