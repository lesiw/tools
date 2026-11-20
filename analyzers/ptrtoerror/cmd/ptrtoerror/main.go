// Command ptrtoerror reports a concrete error type converted to error both as
// a value and as a pointer.
package main

import (
	"lesiw.io/tools/analyzers/ptrtoerror"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(ptrtoerror.Analyzer) }
