// Command fix applies the analyzer suite's fixes, repeating until no fix is
// left.
//
//	go run lesiw.io/tools/cmd/fix@latest [flags] [packages]
//
// The analyzers run in this process. A round loads the packages, runs the
// suite, and writes every edit it suggested; the loop ends when a round finds
// nothing. Build flags go to the go command unchanged, and an analyzer is
// selected by name the way vet selects one: -NAME runs it, -NAME=false turns
// it off, and -NAME.flag=value sets one of its own flags.
package main

import (
	"lesiw.io/tools/analyzers/linelen"
	"lesiw.io/tools/internal/suite"
	"lesiw.io/tools/multichecker"
)

// A command sets the width it runs with; the library default is no limit.
func init() { linelen.Config.Len = 79 }
func main() { multichecker.Fix(suite.Analyzers()...) }
