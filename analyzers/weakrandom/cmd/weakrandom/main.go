// Command weakrandom reports a weak random value used for a token, key, or
// password.
package main

import (
	"lesiw.io/tools/analyzers/weakrandom"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(weakrandom.Analyzer) }
