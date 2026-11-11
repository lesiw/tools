// Command uuid reports a UUID built by formatting hex digits into the
// canonical shape.
package main

import (
	"lesiw.io/tools/analyzers/uuid"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(uuid.Analyzer) }
