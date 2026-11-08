// Command vet checks Go packages with the lesiw.io analyzer suite.
//
//	go tool lesiw.io/tools/cmd/vet [packages]
//
// It loads packages itself and reports diagnostics to standard error. It exits
// non-zero when it reports any diagnostic. The -fix flag applies the suite's
// suggested fixes.
package main

import (
	"os"

	"lesiw.io/tools/analyzers/linelen"
	"lesiw.io/tools/internal/vet"
)

// A command sets the width it runs with; the library default is no limit.
func init() { linelen.Config.Len = 79 }
func main() { vet.Main(os.Args[1:]) }
