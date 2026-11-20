// Package unusedvariable provides an analysis.Analyzer that reports a variable
// that is declared and never read.
//
// Go refuses to compile the package until the variable is gone, so the
// finding adds only the fix, which the compiler does not offer. The README has
// the fixes and the shape the rule leaves alone.
//
// Vendored from gopls' unusedvariable
// (gopls/internal/analysis/unusedvariable, golang.org/x/tools/gopls v0.24.0),
// which is internal and cannot be imported. The rule is upstream's; the
// deletion and the effect test are adapted from the refactor and typesinternal
// packages of golang.org/x/tools/internal.
package unusedvariable

import (
	"strings"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "unusedvariable",
	Doc:  "reports a variable that is declared and never read",
	Run:  run,
	// The diagnostic is a type error, so the pass has to run on a package
	// that does not compile.
	RunDespiteErrors: true,
}

// unusedName parses a type-checker message that reports a variable declared
// and never read. A type switch's guard puts the name at the start of its
// message instead of after a colon.
func unusedName(msg string) (string, bool) {
	if rest, ok := strings.CutPrefix(msg, "declared and not used: "); ok {
		return rest, true
	}
	if name, ok := strings.CutSuffix(msg, " declared and not used"); ok {
		return name, true
	}
	return "", false
}

func run(pass *analysis.Pass) (any, error) {
	if len(pass.TypeErrors) == 0 {
		// The rule only reports what the type checker refused to compile.
		return nil, nil
	}
	src := newSyntax(pass)
	for _, typeErr := range pass.TypeErrors {
		name, ok := unusedName(typeErr.Msg)
		if !ok {
			continue
		}
		id := src.idents[typeErr.Pos]
		if id == nil || id.Name != name {
			continue
		}
		edits := src.deleteVar(id)
		if len(edits) == 0 {
			continue // the declaration is not one this rule removes
		}
		// The writes go with the declaration: they are the mentions that
		// kept it in place.
		edits = append(edits, src.writes(id)...)
		pass.Report(analysis.Diagnostic{
			Pos:     id.Pos(),
			End:     id.End(),
			Message: "variable " + name + " is declared and never read",
			SuggestedFixes: []analysis.SuggestedFix{{
				Message:   "Remove variable " + name,
				TextEdits: edits,
			}},
		})
	}
	return nil, nil
}
