// Command uncheckedinlineerr reports an empty body in an if that checks an
// error from its init.
package main

import (
	"lesiw.io/tools/analyzers/uncheckedinlineerr"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(uncheckedinlineerr.Analyzer) }
