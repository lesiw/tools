// Command nowsub reports time.Now().Sub(t) or t.Sub(time.Now()).
package main

import (
	"lesiw.io/tools/analyzers/nowsub"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(nowsub.Analyzer) }
