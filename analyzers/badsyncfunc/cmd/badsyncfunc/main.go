// Command badsyncfunc reports a sync.Once.Do call whose function reads a
// variable that a loop enclosing the call declares, where the loop reuses the
// Once for every iteration.
package main

import (
	"lesiw.io/tools/analyzers/badsyncfunc"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(badsyncfunc.Analyzer) }
