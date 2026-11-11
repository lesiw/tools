// Command errflow reports an else block that follows a terminating statement.
package main

import (
	"lesiw.io/tools/analyzers/errflow"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(errflow.Analyzer) }
