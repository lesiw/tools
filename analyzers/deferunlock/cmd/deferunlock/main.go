// Command deferunlock reports a deferred Unlock without Lock, or a deferred
// RUnlock without RLock, in the same function.
package main

import (
	"lesiw.io/tools/analyzers/deferunlock"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(deferunlock.Analyzer) }
