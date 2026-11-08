// Command vet checks Go packages with the lesiw.io analyzer suite.
//
//	go tool lesiw.io/tools/cmd/vet [packages]
//
// The go command loads the packages and writes the report; this program is the
// tool it calls. It exits non-zero when the suite reports any diagnostic, and
// the -fix flag applies the suite's suggested fixes.
package main

import (
	"lesiw.io/tools/analyzers/linelen"
	"lesiw.io/tools/multichecker"
)

// A command sets the width it runs with; the library default is no limit.
func init() { linelen.Config.Len = 79 }
func main() { multichecker.Main(analyzers...) }
