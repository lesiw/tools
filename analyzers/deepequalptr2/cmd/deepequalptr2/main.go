// Command deepequalptr2 reports a reflect.DeepEqual call on two pointers,
// including unsafe.Pointer values.
package main

import (
	"lesiw.io/tools/analyzers/deepequalptr2"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(deepequalptr2.Analyzer) }
