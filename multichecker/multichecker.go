// Package multichecker runs a set of analyzers as one vet tool.
//
// It is golang.org/x/tools/go/analysis/multichecker with the //ignore
// directives honored: a diagnostic inside a range a directive covers is
// dropped before the driver sees it. The singlechecker honors the same
// directives, so a standalone command and the suite agree about what a
// directive suppresses.
package multichecker

import (
	"golang.org/x/tools/go/analysis"
	xmultichecker "golang.org/x/tools/go/analysis/multichecker"

	"lesiw.io/tools/internal/ignore"
	"lesiw.io/tools/internal/tool"
)

// Main runs analyzers over the packages named on the command line. It does
// not return.
func Main(analyzers ...*analysis.Analyzer) {
	tool.Main(func() { xmultichecker.Main(ignore.Apply(analyzers...)...) })
}
