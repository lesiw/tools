// Command osremoveall reports os.Remove or os.RemoveAll on a path that an
// empty variable leaves at the filesystem root.
package main

import (
	"lesiw.io/tools/analyzers/osremoveall"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(osremoveall.Analyzer) }
