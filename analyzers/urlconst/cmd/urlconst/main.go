// Command urlconst reports a constant URL that url.Parse rejects.
package main

import (
	"lesiw.io/tools/analyzers/urlconst"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(urlconst.Analyzer) }
