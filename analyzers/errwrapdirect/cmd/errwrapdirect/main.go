// Command errwrapdirect reports a fmt.Errorf call that wraps an error
// without a message.
package main

import (
	"lesiw.io/tools/analyzers/errwrapdirect"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(errwrapdirect.Analyzer) }
