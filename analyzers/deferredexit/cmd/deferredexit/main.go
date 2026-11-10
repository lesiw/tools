// Command deferredexit reports an exit that happens while deferred calls are
// pending.
package main

import (
	"lesiw.io/tools/analyzers/deferredexit"
	"lesiw.io/tools/singlechecker"
)

func main() { singlechecker.Main(deferredexit.Analyzer) }
