// Command finalizercapture reports a finalizer that captures the object it
// finalizes.
package main

import (
	"lesiw.io/tools/analyzers/finalizercapture"
	"lesiw.io/tools/analyzers/linelen"
	"lesiw.io/tools/multichecker"
)

func init() { linelen.Config.Len = 79 }
func main() { multichecker.Main(finalizercapture.Analyzer) }
