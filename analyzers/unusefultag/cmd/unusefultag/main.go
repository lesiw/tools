// Command unusefultag reports a json struct tag not used by encoding/json.
package main

import (
	"lesiw.io/tools/analyzers/unusefultag"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(unusefultag.Analyzer) }
