// Command headercanon reports a non-canonical key in an http.Header map.
package main

import (
	"lesiw.io/tools/analyzers/headercanon"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(headercanon.Analyzer) }
