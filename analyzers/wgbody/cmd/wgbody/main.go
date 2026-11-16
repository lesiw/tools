// Command wgbody reports a Done call or a panic inside a
// sync.WaitGroup.Go body.
package main

import (
	"lesiw.io/tools/analyzers/wgbody"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(wgbody.Analyzer) }
