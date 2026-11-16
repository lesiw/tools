// Command weakrsa reports rsa.GenerateKey with a constant key size below 2048
// bits.
package main

import (
	"lesiw.io/tools/analyzers/weakrsa"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(weakrsa.Analyzer) }
