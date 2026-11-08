// Command modernize runs the modernize analyzers.
package main

import (
	"lesiw.io/tools/analyzers/modernize"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(modernize.Suite...) }
