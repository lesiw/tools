// Command mathrandseed reports a call to math/rand.Seed.
package main

import (
	"lesiw.io/tools/analyzers/mathrandseed"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(mathrandseed.Analyzer) }
