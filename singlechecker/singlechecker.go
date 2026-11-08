// Package singlechecker runs one analyzer as a vet tool.
//
// It is golang.org/x/tools/go/analysis/singlechecker with the //ignore
// directives honored: a diagnostic inside a range a directive covers is
// dropped before the driver sees it. The multichecker honors the same
// directives, so a standalone command and the suite agree about what a
// directive suppresses.
package singlechecker

import (
	"golang.org/x/tools/go/analysis"
	xsinglechecker "golang.org/x/tools/go/analysis/singlechecker"

	"lesiw.io/tools/internal/ignore"
	"lesiw.io/tools/internal/tool"
)

// Main runs analyzer over the packages named on the command line. It does not
// return.
func Main(analyzer *analysis.Analyzer) {
	tool.Main(func() { xsinglechecker.Main(ignore.Apply(analyzer)[0]) })
}
