// Command unexportedreturn reports an exported function or method whose
// result is an unexported type declared in the same package.
package main

import (
	"lesiw.io/tools/analyzers/unexportedreturn"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(unexportedreturn.Analyzer) }
