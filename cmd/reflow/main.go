// Command reflow fills comment prose to the width limit. It also lifts a
// trailing comment that is too long for its line above the code it trails. It
// reads Go source on standard input and writes the result to standard output.
//
// It is a separate command because it changes words. A reflowed paragraph is a
// different document, and an author who breaks lines on purpose does not want
// that.
//
//	go tool lesiw.io/tools/cmd/reflow < in.go > out.go
package main

import (
	"os"

	"lesiw.io/tools/internal/reflow"
)

func main() { os.Exit(reflow.Main(os.Args[1:])) }
