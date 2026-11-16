// Package blankrange provides an analysis.Analyzer that reports a range
// clause with an unnecessary blank identifier.
//
// The section of the spec on for statements says that "if the last iteration
// variable is the blank identifier, the range clause is equivalent to the same
// clause without that identifier" (https://go.dev/ref/spec#For_statements).
// The same section says the range expression is not evaluated when at most
// one iteration variable is present and x or len(x) is constant. Removing a
// second variable from a clause over an array or a pointer to an array can
// therefore skip an evaluation. When removing the blank is not safe, the
// clause is not reported. gofmt -s rewrites the same clause, though a
// parenthesized blank is unchanged; it also rewrites the array clauses this
// check does not report. vet and fix do not apply gofmt -s.
//
// The clause is reported when its second variable is blank, or when its first
// and only variable is blank. The single-variable form takes =, since the
// short declaration requires a new variable. Parentheses around a blank do not
// change the report. The fix writes the clause without the blank. It is
// withheld when the edit would delete a comment.
//
// The rule is inspired by revive's range.
package blankrange

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"

	"lesiw.io/tools/internal/astexpr"
)

var Analyzer = &analysis.Analyzer{
	Name: "blankrange",
	Doc:  "reports a range clause with an unnecessary blank identifier",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			if loop, ok := n.(*ast.RangeStmt); ok {
				check(pass, file, loop)
			}
			return true
		})
	}
	return nil, nil
}

func check(pass *analysis.Pass, file *ast.File, loop *ast.RangeStmt) {
	node := loop.Value
	if node == nil {
		node = loop.Key
	}
	if !blank(node) {
		return
	}
	if forced(pass, loop) {
		return
	}
	diag := analysis.Diagnostic{
		Pos:     node.Pos(),
		End:     node.End(),
		Message: "blank identifier in range clause is unnecessary",
	}
	if fix, ok := drop(pass, file, loop); ok {
		diag.SuggestedFixes = []analysis.SuggestedFix{fix}
	}
	pass.Report(diag)
}

func drop(pass *analysis.Pass, file *ast.File, loop *ast.RangeStmt) (analysis.SuggestedFix, bool) {
	var (
		pos     token.Pos
		newText string
	)
	if blank(loop.Key) {
		pos = loop.Key.Pos()
		if needsSpace(pass, pos) {
			newText = " "
		}
	} else {
		pos = loop.Key.End()
		newText = " " + loop.Tok.String() + " "
	}
	if commented(file, pos, loop.Range) {
		return analysis.SuggestedFix{}, false
	}
	message := "Drop the blank identifier"
	if loop.Value != nil && blank(loop.Key) {
		message = "Drop the blank identifiers"
	}
	return analysis.SuggestedFix{
		Message: message,
		TextEdits: []analysis.TextEdit{{
			Pos:     pos,
			End:     loop.Range,
			NewText: []byte(newText),
		}},
	}, true
}

func forced(pass *analysis.Pass, loop *ast.RangeStmt) bool {
	return loop.Value != nil && constantLen(pass, loop.X) &&
		!arraySafe(pass, loop.X)
}

// constantLen reports whether len(e) is a constant: e has an array or a
// pointer-to-array type and contains no channel receive or non-constant call.
func constantLen(pass *analysis.Pass, e ast.Expr) bool {
	if !arraylike(pass, e) {
		return false
	}
	constant := true
	ast.Inspect(e, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit:
			// A function literal creates a closure; its body runs later.
			return false
		case *ast.CallExpr:
			if !astexpr.Pure(pass, n) && pass.TypesInfo.Types[n].Value == nil {
				constant = false
			}
		case *ast.UnaryExpr:
			if n.Op == token.ARROW {
				constant = false
			}
		}
		return constant
	})
	return constant
}

// arraySafe reports whether evaluating e can neither panic nor produce a side
// effect. It adds two checks that astexpr.Safe leaves out: a method value on a
// nil interface, and a map literal with an unhashable key.
func arraySafe(pass *analysis.Pass, e ast.Expr) bool {
	if !astexpr.Safe(pass, e) {
		return false
	}
	safe := true
	ast.Inspect(e, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.SelectorExpr:
			sel, ok := pass.TypesInfo.Selections[n]
			if !ok || sel.Kind() != types.MethodVal {
				break
			}
			if _, ok := sel.Recv().Underlying().(*types.Interface); ok {
				safe = false
			}
		case *ast.CompositeLit:
			t := pass.TypesInfo.TypeOf(n)
			if t == nil {
				break
			}
			m, ok := t.Underlying().(*types.Map)
			if ok && !astexpr.NoInterface(m.Key()) {
				safe = false
			}
		}
		return safe
	})
	return safe
}

// needsSpace reports whether pos follows a byte that is not whitespace.
func needsSpace(pass *analysis.Pass, pos token.Pos) bool {
	f := pass.Fset.File(pos)
	if f == nil {
		return true
	}
	src, err := pass.ReadFile(f.Name())
	if err != nil {
		return true
	}
	off := f.Offset(pos)
	if off <= 0 || off > len(src) {
		return true
	}
	switch src[off-1] {
	case ' ', '\t', '\n', '\r':
		return false
	}
	return true
}

// An array or a pointer to an array is evaluated by a clause with two
// variables, and a clause with one variable and a constant length does not
// evaluate it.
func arraylike(pass *analysis.Pass, x ast.Expr) bool {
	t := pass.TypesInfo.TypeOf(x)
	if t == nil {
		return true
	}
	if ptr, ok := t.Underlying().(*types.Pointer); ok {
		_, ok = ptr.Elem().Underlying().(*types.Array)
		return ok
	}
	_, ok := t.Underlying().(*types.Array)
	return ok
}

func blank(e ast.Expr) bool {
	id, ok := ast.Unparen(e).(*ast.Ident)
	return ok && id.Name == "_"
}

// A comment in the span an edit replaces would be lost.
func commented(file *ast.File, pos, end token.Pos) bool {
	for _, group := range file.Comments {
		if group.Pos() < end && group.End() > pos {
			return true
		}
	}
	return false
}
