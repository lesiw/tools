// Command syncmaploaddelete reports a sync.Map load followed by a delete of
// the same key.
package main

import (
	"lesiw.io/tools/analyzers/linelen"
	"lesiw.io/tools/analyzers/syncmaploaddelete"
	"lesiw.io/tools/multichecker"
)

func init() { linelen.Config.Len = 79 }
func main() { multichecker.Main(syncmaploaddelete.Analyzer) }
