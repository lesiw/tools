// Command packagename reports a meaningless package name, a name with an
// underscore, or a name with a capital letter.
package main

import (
	"lesiw.io/tools/analyzers/packagename"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(packagename.Analyzer) }
