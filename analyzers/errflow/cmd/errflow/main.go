// Command errflow reports confusing or inconsistent error flows.
package main

import (
	"lesiw.io/tools/analyzers/errflow"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(errflow.Analyzer) }
