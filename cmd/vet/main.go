// Command vet checks Go packages with the lesiw.io analyzer suite.
//
//	go tool lesiw.io/tools/cmd/vet [packages]
//
// It reports diagnostics to standard error and exits non-zero when there are
// any. The -fix flag applies the suite's suggested fixes.
package main

import (
	"lesiw.io/tools/analyzers/linelen"
	"lesiw.io/tools/internal/suite"
	"lesiw.io/tools/multichecker"
)

// A command sets the width it runs with; the library default is no limit.
func init() { linelen.Config.Len = 79 }
func main() { multichecker.Main(suite.Analyzers()...) }
