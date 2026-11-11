// Command afterhttperror reports an http.Error call without a return
// after it.
package main

import (
	"lesiw.io/tools/analyzers/afterhttperror"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(afterhttperror.Analyzer) }
