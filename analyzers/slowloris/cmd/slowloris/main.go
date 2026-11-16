// Command slowloris reports an http.Server literal with no timeout for reading
// headers.
package main

import (
	"lesiw.io/tools/analyzers/slowloris"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(slowloris.Analyzer) }
