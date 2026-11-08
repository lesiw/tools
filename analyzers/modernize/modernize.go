// Package modernize provides analyzers that rewrite a shape into the one Go
// has since grown a spelling for.
//
// Each analyzer here reports a rewrite the upstream suite does not carry: a
// hand-rolled encoding of something the standard library now does, or two
// calls where one would answer both questions. None of them offers a fix, so
// each is a report rather than a rewrite. They are grouped in one package
// because they are one kind of thing, not because they are related.
package modernize

import (
	"slices"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/modernize"
)

// Suite is every rewrite the modernize command runs: the upstream suite, and
// the rules this package adds to it.
var Suite = slices.Concat(modernize.Suite, []*analysis.Analyzer{
	ContainsCut,
	UUID,
})
