// Command fix applies the analyzer suite's fixes, repeating until the suite
// has nothing left to suggest.
//
//	go run lesiw.io/tools/cmd/fix@latest [flags] [packages]
//
// The go command does the work, one analyzer at a time. Flags go to it
// unchanged, so -NAME runs one analyzer and -NAME=false turns one off. The
// suite named by -vettool defaults to this module's, and is looked for as a
// module tool, then on PATH, and then as the package at its latest version.
package main

import (
	"os"

	"lesiw.io/tools/fix"
)

func main() { os.Exit(fix.Main(os.Args[1:])) }
