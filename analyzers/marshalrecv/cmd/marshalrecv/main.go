// Command marshalrecv reports a marshal method that writes to a value
// receiver.
package main

import (
	"lesiw.io/tools/analyzers/marshalrecv"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(marshalrecv.Analyzer) }
