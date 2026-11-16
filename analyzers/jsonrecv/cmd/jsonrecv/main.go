// Command jsonrecv reports MarshalJSON on a pointer receiver that does not
// need the pointer and a direct UnmarshalJSON write to a value receiver.
package main

import (
	"lesiw.io/tools/analyzers/jsonrecv"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(jsonrecv.Analyzer) }
