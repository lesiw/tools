// Command nilchansend reports a send or receive on a channel that is nil at
// that point.
package main

import (
	"lesiw.io/tools/analyzers/nilchansend"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(nilchansend.Analyzer) }
