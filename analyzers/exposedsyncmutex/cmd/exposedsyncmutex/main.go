// Command exposedsyncmutex reports an embedded lock in a reachable type.
package main

import (
	"lesiw.io/tools/analyzers/exposedsyncmutex"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(exposedsyncmutex.Analyzer) }
