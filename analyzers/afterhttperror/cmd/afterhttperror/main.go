// Command afterhttperror reports an http.Error call without a return
// after it.
package main

import (
	"lesiw.io/tools/analyzers/afterhttperror"
	"lesiw.io/tools/singlechecker"
)

func main() { singlechecker.Main(afterhttperror.Analyzer) }
