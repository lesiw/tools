// Command dupbranch reports an if whose two branches hold the same statements.
package main

import (
	"lesiw.io/tools/analyzers/dupbranch"
	"lesiw.io/tools/singlechecker"
)

func main() { singlechecker.Main(dupbranch.Analyzer) }
