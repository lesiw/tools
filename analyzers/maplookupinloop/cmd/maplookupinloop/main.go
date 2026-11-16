// Command maplookupinloop reports a map lookup in a loop whose map and key do
// not change.
package main

import (
	"lesiw.io/tools/analyzers/maplookupinloop"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(maplookupinloop.Analyzer) }
