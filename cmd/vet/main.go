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

	"lesiw.io/tools/internal/vet"
)

func main() { vet.Main(os.Args[1:]) }
