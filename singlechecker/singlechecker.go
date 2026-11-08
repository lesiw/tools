// Package singlechecker runs one analyzer as a command.
//
// It is golang.org/x/tools/go/analysis/singlechecker with the //ignore
// directives honored and generated files left alone: a diagnostic inside a
// range a directive covers is dropped, and the rule never sees what a
// generator wrote. The multichecker does the same for a set, so a standalone
// command and the suite agree about what a directive suppresses.
package singlechecker

import (
	"golang.org/x/tools/go/analysis"
	xsinglechecker "golang.org/x/tools/go/analysis/singlechecker"

	"lesiw.io/tools/internal/ignore"
)

// Main runs analyzer over the packages named on the command line. It does not
// return.
func Main(analyzer *analysis.Analyzer) {
	xsinglechecker.Main(ignore.Apply(analyzer)[0])
}
