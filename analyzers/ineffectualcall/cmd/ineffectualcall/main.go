// Command ineffectualcall reports a call statement to a value-receiver method
// that only returns an effect-free value.
package main

import (
	"lesiw.io/tools/analyzers/ineffectualcall"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(ineffectualcall.Analyzer) }
