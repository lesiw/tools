// Command taggedswitch reports an untagged switch that compares one expression
// with a series of values.
package main

import (
	"lesiw.io/tools/analyzers/linelen"
	"lesiw.io/tools/analyzers/taggedswitch"
	"lesiw.io/tools/multichecker"
)

// A command sets the width it runs with; the library default is no limit.
func init() { linelen.Config.Len = 79 }
func main() { multichecker.Main(taggedswitch.Analyzer) }
