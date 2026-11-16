// Command errwrapnil reports a fmt.Errorf %w verb with a nil error operand.
package main

import (
	"lesiw.io/tools/analyzers/errwrapnil"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(errwrapnil.Analyzer) }
