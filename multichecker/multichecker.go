// Package multichecker runs a set of analyzers as one command.
//
// It is golang.org/x/tools/go/analysis/multichecker with the //ignore
// directives honored and generated files left alone: a diagnostic inside a
// range a directive covers is dropped, and a rule never sees what a generator
// wrote. The singlechecker does the same for one analyzer, so a standalone
// command and the suite agree about what a directive suppresses.
package multichecker

import (
	"golang.org/x/tools/go/analysis"
	xmultichecker "golang.org/x/tools/go/analysis/multichecker"

	"lesiw.io/tools/internal/ignore"
)

// Main runs analyzers over the packages named on the command line. It does
// not return.
func Main(analyzers ...*analysis.Analyzer) {
	xmultichecker.Main(ignore.Apply(analyzers...)...)
}
