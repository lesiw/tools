// Command strreplaceall reports a strings.Replace call with a count of -1.
package main

import (
	"lesiw.io/tools/analyzers/strreplaceall"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(strreplaceall.Analyzer) }
