// Command impossibleplatform reports a comparison of runtime.GOOS or
// runtime.GOARCH with a value the file's build constraints rule out.
package main

import (
	"lesiw.io/tools/analyzers/impossibleplatform"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(impossibleplatform.Analyzer) }
