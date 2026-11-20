// Command embeddirective reports a file with a //go:embed directive and no
// embed import, and a directive that does not precede one variable of type
// string, []byte or embed.FS with no value.
package main

import (
	"lesiw.io/tools/analyzers/embeddirective"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(embeddirective.Analyzer) }
