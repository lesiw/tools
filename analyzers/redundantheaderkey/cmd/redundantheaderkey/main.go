// Command redundantheaderkey reports a redundant http.CanonicalHeaderKey on
// the key argument of an http.Header method.
package main

import (
	"lesiw.io/tools/analyzers/redundantheaderkey"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(redundantheaderkey.Analyzer) }
