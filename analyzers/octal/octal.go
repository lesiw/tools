// Package octal provides an analysis.Analyzer that reports an octal literal
// written with a leading zero instead of the 0o prefix.
//
// Go 1.13 added the prefix, and both spellings compile to the same value. The
// older spelling marks the base with a leading zero, which a reader expects to
// be padding, so 0755 is easy to read as decimal. The proposal that added the
// prefix (https://go.dev/design/19308-number-literals) left the rewrite for
// later: "after a few years, once there are no supported Go releases missing
// the 0o377 syntax, we could consider changing gofmt to at least reformat 0377
// to 0o377 for clarity." The suggested fix applies that rewrite to a literal.
//
// Such a literal is not usually a defect, so this is a house rule: the suite's
// own code writes the prefix, and the fix changes only the spelling.
//
// An argument of a call that resolves to time.Date keeps its zero, because the
// arguments are the fields of one date and a padded field lines up with the
// ones beside it. The exemption covers every position, so it reaches the year
// and the nanosecond; a literal inside an argument expression is reported. The
// README catalogs every check, exemption, and fix.
//
// The rule is like go-critic's octalLiteral.
package octal

import (
	"go/ast"
	"go/token"
	"go/version"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/types/typeutil"
)

var Analyzer = &analysis.Analyzer{
	Name: "octal",
	Doc:  "reports an octal literal written with a leading zero",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		if !prefixOK(pass.TypesInfo.FileVersions[file]) {
			continue
		}
		// The walk visits a call before the literals it holds, so an
		// argument is marked by the time its literal arrives.
		dateArgs := make(map[token.Pos]struct{})
		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.CallExpr:
				if !isTimeDate(pass, n) {
					return true
				}
				for _, arg := range n.Args {
					if lit, ok := ast.Unparen(arg).(*ast.BasicLit); ok {
						dateArgs[lit.Pos()] = struct{}{}
					}
				}
			case *ast.BasicLit:
				if !flagged(n) {
					return true
				}
				if _, ok := dateArgs[n.Pos()]; ok {
					return true
				}
				pass.Report(analysis.Diagnostic{
					Pos:     n.Pos(),
					End:     n.End(),
					Message: n.Value + " can be replaced by 0o" + n.Value[1:],
					SuggestedFixes: []analysis.SuggestedFix{{
						Message: "Add the 0o prefix",
						TextEdits: []analysis.TextEdit{{
							Pos:     n.Pos() + 1,
							End:     n.Pos() + 1,
							NewText: []byte("o"),
						}},
					}},
				})
			}
			return true
		})
	}
	return nil, nil
}

// prefixOK reports whether goVersion accepts the 0o prefix. The empty string
// means the type checker was given no version, and it is treated as modern.
// go/types records a file that names a version as at least go1.21, because
// that is the first release that reads a version from a build line, so only a
// version inherited from the module falls below the boundary.
func prefixOK(goVersion string) bool {
	return goVersion == "" || version.Compare(goVersion, "go1.13") >= 0
}

func flagged(lit *ast.BasicLit) bool {
	value := lit.Value
	if lit.Kind != token.INT || value[0] != '0' {
		return false
	}
	for i := 1; i < len(value); i++ {
		switch b := value[i]; {
		case b == '_':
			// A separator there sets the zero apart from the digits, so
			// they do not read as a padded field.
			if i == 1 {
				return false
			}
		case b < '0' || b > '7':
			return false
		}
	}
	return strings.ContainsAny(value[1:], "1234567")
}

// isTimeDate reports whether call is a call to time.Date. The exemption keys
// on the call because a type test would catch the month alone: only the month
// is a time.Month, and every other position that holds a literal is an int.
// StaticCallee is what sees a dot-imported or parenthesized callee, which the
// path a selector's package name gives cannot.
func isTimeDate(pass *analysis.Pass, call *ast.CallExpr) bool {
	fn := typeutil.StaticCallee(pass.TypesInfo, call)
	return fn != nil && fn.FullName() == "time.Date"
}
