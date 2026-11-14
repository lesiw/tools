// Command shellcommand reports a program name with whitespace in exec.Command
// or exec.CommandContext.
package main

import (
	"lesiw.io/tools/analyzers/shellcommand"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(shellcommand.Analyzer) }
