// Command weaktls reports a tls.Config literal that disables certificate
// verification or permits TLS below 1.2.
package main

import (
	"lesiw.io/tools/analyzers/weaktls"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(weaktls.Analyzer) }
