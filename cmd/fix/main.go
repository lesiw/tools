// Command fix applies the analyzer suite's fixes, repeating until the tool
// has nothing left to suggest.
//
//	go run lesiw.io/tools/cmd/fix@latest [-vettool prog] [packages]
//
// It runs the analysis tool named by -vettool, which defaults to this module's
// suite. The tool is looked for as a module tool, then on PATH, and then as
// the package at its latest version. A tree that has added nothing still
// works.
package main

import (
	"os"

	"lesiw.io/tools/internal/fix"
)

func main() { os.Exit(fix.Main(os.Args[1:])) }
