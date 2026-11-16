// Command regexpraw reports a regexp pattern written as an interpreted
// string that doubles every backslash and holds at least one.
package main

import (
	"lesiw.io/tools/analyzers/regexpraw"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(regexpraw.Analyzer) }
