// Command todocomment reports a TODO, FIXME or BUG doc comment with no detail.
package main

import (
	"lesiw.io/tools/analyzers/todocomment"
	"lesiw.io/tools/singlechecker"
)

func main() { singlechecker.Main(todocomment.Analyzer) }
