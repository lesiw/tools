// Command structconvert reports a struct literal that copies every field of
// another struct.
package main

import (
	"lesiw.io/tools/analyzers/linelen"
	"lesiw.io/tools/analyzers/structconvert"
	"lesiw.io/tools/multichecker"
)

func init() { linelen.Config.Len = 79 }
func main() { multichecker.Main(structconvert.Analyzer) }
