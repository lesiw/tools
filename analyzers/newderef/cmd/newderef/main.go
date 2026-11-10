// Command newderef reports a dereference of a fresh allocation.
package main

import (
	"lesiw.io/tools/analyzers/newderef"
	"lesiw.io/tools/singlechecker"
)

func main() { singlechecker.Main(newderef.Analyzer) }
