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

	"lesiw.io/tools/internal/fmt"
)

func main() { os.Exit(fmt.Main(os.Args[1:])) }
