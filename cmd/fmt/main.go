// Command fmt formats Go source under stricter layout rules than gofmt. Run it
// after gofmt and goimports, per the go.fmt convention. Type-aware rules live
// in the analyzer suite.
//
// It takes gofmt's arguments. It reads standard input when there are no
// arguments, and files and directories otherwise. The -l flag lists what
// differs, and -w rewrites in place.
package main

import (
	"os"

	"lesiw.io/tools/analyzers/linelen"
	"lesiw.io/tools/internal/fmt"
)

// A tool that was asked to run gives the width rules the house limit; the
// library default is no limit at all.
func init() { linelen.Config.Len = 79 }

func main() { os.Exit(fmt.Main(os.Args[1:])) }
