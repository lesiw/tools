// Command uncatchablesignal reports a signal.Notify or signal.NotifyContext
// call that names SIGKILL or SIGSTOP.
package main

import (
	"lesiw.io/tools/analyzers/uncatchablesignal"
	"lesiw.io/tools/multichecker"
)

func main() { multichecker.Main(uncatchablesignal.Analyzer) }
