// Command wgogoroutine reports a WaitGroup count raised from inside a
// goroutine instead of before it starts.
package main

import (
	"lesiw.io/tools/analyzers/wgogoroutine"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(wgogoroutine.Analyzer) }
